// Package notify is a loopback-only listener that Claude Code hooks (via
// `kaze notify`) use to tell kaze a tab is done or needs the user.
package notify

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"sync"
	"time"
)

type Kind string

const (
	KindDone  Kind = "done"
	KindNeeds Kind = "needs"
)

const (
	maxBody      = 4 * 1024
	readDeadline = 2 * time.Second
)

type Msg struct {
	Tab    int    `json:"tab"`
	Kind   Kind   `json:"kind"`
	Reason string `json:"reason,omitempty"`
	Token  string `json:"token"`
}

type Server struct {
	ln    net.Listener
	token string
	onMsg func(Msg)
	wg    sync.WaitGroup
}

// Listen binds 127.0.0.1 on an OS-chosen port. onMsg runs on a per-connection
// goroutine; the caller must hop to its own goroutine.
func Listen(onMsg func(Msg)) (*Server, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{ln: ln, token: hex.EncodeToString(raw[:]), onMsg: onMsg}
	s.wg.Add(1)
	go s.accept()
	return s, nil
}

func (s *Server) Addr() string  { return s.ln.Addr().String() }
func (s *Server) Token() string { return s.token }

func (s *Server) Close() error {
	err := s.ln.Close()
	s.wg.Wait()
	return err
}

func (s *Server) accept() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		// Own goroutine so a silent client cannot stall the others.
		go func() {
			defer s.wg.Done()
			s.handle(c)
		}()
	}
}

func (s *Server) handle(c net.Conn) {
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(readDeadline))
	// One extra byte tells us the body is over the cap.
	b, err := io.ReadAll(io.LimitReader(c, maxBody+1))
	if err != nil || len(b) > maxBody {
		return
	}
	var m Msg
	if json.Unmarshal(b, &m) != nil {
		return
	}
	if subtle.ConstantTimeCompare([]byte(m.Token), []byte(s.token)) != 1 {
		return
	}
	if m.Kind != KindDone && m.Kind != KindNeeds {
		return
	}
	m.Token = ""
	s.onMsg(m)
}

// Send delivers one message; used by `kaze notify`.
func Send(addr, token string, m Msg, timeout time.Duration) error {
	m.Token = token
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(timeout))
	if _, err := c.Write(b); err != nil {
		return err
	}
	if tc, ok := c.(*net.TCPConn); ok {
		return tc.CloseWrite() // EOF marks end of the single JSON object
	}
	return nil
}
