package main

import (
	"encoding/json"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
)

const (
	stdinCap       = 64 << 10
	stdinWait      = 300 * time.Millisecond
	notifyTotal    = time.Second // Claude Code waits on its hooks, so never take longer
	reasonMaxRunes = 120
)

// runNotify is `kaze notify <done|needs>`, run by Claude Code hooks. It always
// returns 0 and prints nothing: an alert is a nicety and must never fail or
// slow the hook.
func runNotify(args []string, stdin io.Reader, env func(string) string) (code int) {
	// Exit 2 from a hook is a blocking error for Claude Code, so a panic must still exit 0.
	defer func() {
		if recover() != nil {
			code = 0
		}
	}()
	start := time.Now()
	if len(args) != 1 {
		return 0
	}
	kind := notify.Kind(args[0])
	if kind != notify.KindDone && kind != notify.KindNeeds {
		return 0
	}
	addr, token := env("KAZE_NOTIFY"), env("KAZE_NOTIFY_TOKEN")
	tab, err := strconv.Atoi(env("KAZE_TAB_ID"))
	if addr == "" || token == "" || err != nil || tab <= 0 {
		return 0 // not run from a kaze tab
	}
	msg := notify.Msg{Tab: tab, Kind: kind}
	if stdin != nil {
		var hook struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(readStdinLimited(stdin, stdinWait), &hook) == nil {
			msg.Reason = trimRunes(hook.Message, reasonMaxRunes)
		}
	}
	if left := notifyTotal - time.Since(start); left > 0 {
		_ = notify.Send(addr, token, msg, left)
	}
	return 0
}

// readStdinLimited returns what arrived within wait, at most stdinCap bytes. The
// reader goroutine may outlive the call when the pipe never closes; the process
// exits right after, so that is fine.
func readStdinLimited(r io.Reader, wait time.Duration) []byte {
	var (
		mu  sync.Mutex
		buf []byte
	)
	done := make(chan struct{})
	go func() {
		defer close(done)
		chunk := make([]byte, 4096)
		for {
			n, err := r.Read(chunk)
			mu.Lock()
			room := stdinCap - len(buf)
			buf = append(buf, chunk[:min(n, room)]...)
			full := len(buf) >= stdinCap
			mu.Unlock()
			if err != nil || full {
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(wait):
	}
	mu.Lock()
	defer mu.Unlock()
	return append([]byte(nil), buf...)
}

func trimRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// stdinForNotify is nil when stdin is a terminal or missing (windowsgui builds
// may have no console), so nothing waits on a keyboard.
func stdinForNotify() io.Reader {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice != 0 {
		return nil
	}
	return os.Stdin
}
