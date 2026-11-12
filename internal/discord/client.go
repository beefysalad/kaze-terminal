package discord

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// ClientID is the owner's Discord Application ID. Empty disables presence.
const ClientID = ""

// Options supplies the local transport and clock. Zero timing values use defaults.
type Options struct {
	ClientID string
	LockPath string
	Dial     func() (io.ReadWriteCloser, error)
	Now      func() time.Time
	After    func(time.Duration) <-chan time.Time
	Pid      int
	Spacing  time.Duration
	Reply    time.Duration
}

// Client keeps only the latest requested activity. Its worker owns protocol I/O.
// Transports must unblock pending I/O when closed, as the platform dialers do.
type Client struct {
	o       Options
	mu      sync.Mutex
	wanted  *Activity
	closed  bool
	conn    io.ReadWriteCloser
	release func()
	wake    chan struct{}
	stop    chan struct{}
	done    chan struct{}
	nonce   uint64 // worker only
}

func New(o Options) *Client {
	if o.Dial == nil {
		o.Dial = Dial
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.After == nil {
		o.After = time.After
	}
	if o.Pid == 0 {
		o.Pid = os.Getpid()
	}
	if o.Spacing <= 0 {
		o.Spacing = 15 * time.Second
	}
	if o.Reply <= 0 {
		o.Reply = 5 * time.Second
	}
	c := &Client{o: o, wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	go c.run()
	return c
}

// Set copies the activity and wakes the worker without doing transport I/O.
func (c *Client) Set(a *Activity) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.wanted = nil
	if a != nil {
		copy := *a
		c.wanted = &copy
	}
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Close cancels I/O and relinquishes ownership without waiting for the worker
// or sending a clear. Closing the socket removes Discord's card.
func (c *Client) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.stop)
	conn, release := c.conn, c.release
	c.conn = nil
	c.release = nil
	c.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
	if release != nil {
		release()
	}
}
func (c *Client) snapshot() (*Activity, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wanted, c.closed
}
func (c *Client) hold(release func()) bool {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		release()
		return false
	}
	c.release = release
	c.mu.Unlock()
	return true
}
func (c *Client) attach(conn io.ReadWriteCloser) bool {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		conn.Close()
		return false
	}
	c.conn = conn
	c.mu.Unlock()
	return true
}
func (c *Client) drop() {
	c.mu.Lock()
	conn, release := c.conn, c.release
	c.conn = nil
	c.release = nil
	c.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
	if release != nil {
		release()
	}
}

func (c *Client) run() {
	defer close(c.done)
	defer c.drop()
	if c.o.ClientID == "" {
		return
	}
	var retry time.Time
	var retryTick <-chan time.Time
	for {
		wanted, closed := c.snapshot()
		if closed {
			return
		}
		if wanted == nil {
			select {
			case <-c.stop:
				return
			case <-c.wake:
			}
			continue
		}
		if delay := retry.Sub(c.o.Now()); delay > 0 {
			if retryTick == nil {
				retryTick = c.o.After(delay)
			}
			// The injected clock may advance between Now and After.
			if !retry.After(c.o.Now()) {
				continue
			}
			select {
			case <-c.stop:
				return
			case <-c.wake:
			case <-retryTick:
				retryTick = nil
			}
			continue
		}
		// Even failure to take ownership counts as an attempt.
		retry = c.o.Now().Add(c.o.Spacing)
		retryTick = nil
		release := func() {}
		if c.o.LockPath != "" {
			var err error
			release, err = lockFile(c.o.LockPath)
			if err != nil {
				continue
			}
		}
		if !c.hold(release) {
			return
		}
		if wanted, closed = c.snapshot(); closed || wanted == nil {
			c.drop()
			continue
		}
		conn, err := c.o.Dial()
		if err != nil {
			if conn != nil {
				conn.Close()
			}
			c.drop()
			continue
		}
		if conn == nil {
			c.drop()
			continue
		}
		if !c.attach(conn) {
			return
		}
		c.session(conn)
		c.drop()
		retry = c.o.Now().Add(c.o.Spacing)
		retryTick = nil
	}
}

type incoming struct {
	op   uint32
	body []byte
	err  error
}

func (c *Client) session(conn io.ReadWriteCloser) {
	frames := make(chan incoming, 1)
	readStop := make(chan struct{})
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			op, b, err := readFrame(conn)
			select {
			case frames <- incoming{op, b, err}:
			case <-readStop:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	defer func() { close(readStop); conn.Close(); <-readDone }()
	handshake, _ := json.Marshal(map[string]any{"v": 1, "client_id": c.o.ClientID})
	// One allowance covers both a blocked handshake write and waiting for READY.
	readyTimer := c.o.After(c.o.Reply)
	if c.writeUntil(conn, opHandshake, handshake, readyTimer) != nil {
		return
	}
	for {
		select {
		case <-c.stop:
			return
		case <-c.wake:
			if a, _ := c.snapshot(); a == nil {
				return
			}
		case <-readyTimer:
			return
		case f := <-frames:
			var ready struct {
				Evt string `json:"evt"`
			}
			if f.err != nil || f.op != opFrame || json.Unmarshal(f.body, &ready) != nil || ready.Evt != "READY" {
				return
			}
			c.connected(conn, frames)
			return
		}
	}
}

func (c *Client) connected(conn io.ReadWriteCloser, frames <-chan incoming) {
	var sent *Activity
	var nextSend time.Time
	var tick <-chan time.Time
	for {
		wanted, closed := c.snapshot()
		if closed {
			return
		}
		if wanted == nil {
			c.write(conn, opFrame, c.command(nil))
			return
		}
		if sent == nil || *wanted != *sent {
			delay := nextSend.Sub(c.o.Now())
			if delay <= 0 {
				if c.write(conn, opFrame, c.command(wanted)) != nil {
					return
				}
				copy := *wanted
				sent = &copy
				nextSend = c.o.Now().Add(c.o.Spacing)
				tick = nil
				continue
			}
			if tick == nil {
				tick = c.o.After(delay)
			}
			if !nextSend.After(c.o.Now()) {
				continue
			}
		}
		select {
		case <-c.stop:
			return
		case <-c.wake:
		case <-tick:
			tick = nil
		case f := <-frames:
			if f.err != nil || f.op == opClose {
				return
			}
			if f.op == opPing && c.write(conn, opPong, f.body) != nil {
				return
			}
		}
	}
}

func (c *Client) command(a *Activity) []byte {
	c.nonce++
	args := map[string]any{"pid": c.o.Pid}
	if a != nil {
		args["activity"] = a.payload()
	}
	b, _ := json.Marshal(map[string]any{"cmd": "SET_ACTIVITY", "args": args, "nonce": fmt.Sprintf("%d-%d", c.o.Pid, c.nonce)})
	return b
}

// A write gets the same bounded allowance as READY. The watchdog closes the
// transport on timeout or shutdown, then joins its canceled writer.
func (c *Client) write(conn io.ReadWriteCloser, op uint32, body []byte) error {
	return c.writeUntil(conn, op, body, c.o.After(c.o.Reply))
}

func (c *Client) writeUntil(conn io.ReadWriteCloser, op uint32, body []byte, timeout <-chan time.Time) error {
	done := make(chan error, 1)
	go func() { done <- writeFrame(conn, op, body) }()
	for {
		select {
		case err := <-done:
			return err
		case <-c.stop:
			conn.Close()
			<-done
			return io.ErrClosedPipe
		case <-timeout:
			conn.Close()
			<-done
			return errors.New("discord: write timeout")
		}
	}
}
