package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
)

func startServer(t *testing.T) (*notify.Server, chan notify.Msg) {
	t.Helper()
	got := make(chan notify.Msg, 4)
	srv, err := notify.Listen(func(m notify.Msg) { got <- m })
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv, got
}

func envOf(srv *notify.Server, tab string) func(string) string {
	m := map[string]string{"KAZE_NOTIFY": srv.Addr(), "KAZE_NOTIFY_TOKEN": srv.Token(), "KAZE_TAB_ID": tab}
	return func(k string) string { return m[k] }
}

func wait(t *testing.T, ch chan notify.Msg) notify.Msg {
	t.Helper()
	select {
	case m := <-ch:
		return m
	case <-time.After(3 * time.Second):
		t.Fatal("no message received")
		return notify.Msg{}
	}
}

func TestNotifyNeedsSendsReasonFromHookJSON(t *testing.T) {
	srv, got := startServer(t)
	in := strings.NewReader(`{"hook_event_name":"Notification","message":"Claude needs your permission to use Bash"}`)
	if code := runNotify([]string{"needs"}, in, envOf(srv, "3")); code != 0 {
		t.Fatalf("exit %d", code)
	}
	m := wait(t, got)
	if m.Tab != 3 || m.Kind != notify.KindNeeds || m.Reason != "Claude needs your permission to use Bash" {
		t.Fatalf("got %+v", m)
	}
}

func TestNotifyDoneSends(t *testing.T) {
	srv, got := startServer(t)
	if code := runNotify([]string{"done"}, strings.NewReader(""), envOf(srv, "2")); code != 0 {
		t.Fatalf("exit %d", code)
	}
	m := wait(t, got)
	if m.Tab != 2 || m.Kind != notify.KindDone || m.Reason != "" {
		t.Fatalf("got %+v", m)
	}
}

func TestNotifyReasonTrimmedTo120Runes(t *testing.T) {
	srv, got := startServer(t)
	in := strings.NewReader(`{"message":"` + strings.Repeat("あ", 300) + `"}`)
	runNotify([]string{"needs"}, in, envOf(srv, "1"))
	if n := len([]rune(wait(t, got).Reason)); n != 120 {
		t.Fatalf("reason has %d runes, want 120", n)
	}
}

func TestNotifyWithoutServerExitsQuietly(t *testing.T) {
	srv, _ := startServer(t)
	dead := envOf(srv, "1")
	srv.Close() // the port is now dead
	for name, env := range map[string]func(string) string{
		"no env":    func(string) string { return "" },
		"dead port": dead,
	} {
		start := time.Now()
		if code := runNotify([]string{"done"}, strings.NewReader("{}"), env); code != 0 {
			t.Errorf("%s: exit %d", name, code)
		}
		if d := time.Since(start); d > 1500*time.Millisecond {
			t.Errorf("%s: took %v", name, d)
		}
	}
}

func TestNotifyBadKindExitsQuietly(t *testing.T) {
	srv, got := startServer(t)
	for _, args := range [][]string{nil, {"bogus"}} {
		if code := runNotify(args, strings.NewReader(""), envOf(srv, "1")); code != 0 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	select {
	case m := <-got:
		t.Fatalf("unexpected message %+v", m)
	case <-time.After(200 * time.Millisecond):
	}
}

// A hook whose stdin never closes must not hang kaze notify.
func TestNotifyStdinThatNeverEndsDoesNotBlock(t *testing.T) {
	srv, got := startServer(t)
	pr, pw := io.Pipe()
	defer pw.Close()
	start := time.Now()
	runNotify([]string{"done"}, pr, envOf(srv, "4"))
	if d := time.Since(start); d > 1500*time.Millisecond {
		t.Fatalf("took %v", d)
	}
	if m := wait(t, got); m.Tab != 4 {
		t.Fatalf("got %+v", m)
	}
}

func TestReadStdinCapsAt64KB(t *testing.T) {
	b := readStdinLimited(bytes.NewReader(make([]byte, 200<<10)), time.Second)
	if len(b) != 64<<10 {
		t.Fatalf("read %d bytes", len(b))
	}
}

func TestNotifyPanicStillExitsZero(t *testing.T) {
	boom := func(string) string { panic("boom") }
	if code := runNotify([]string{"done"}, strings.NewReader(""), boom); code != 0 {
		t.Fatalf("exit %d, want 0 (exit 2 would block Claude Code)", code)
	}
}

func TestNotifyRejectsNonPositiveTab(t *testing.T) {
	srv, got := startServer(t)
	for _, tab := range []string{"0", "-1"} {
		if code := runNotify([]string{"done"}, strings.NewReader(""), envOf(srv, tab)); code != 0 {
			t.Errorf("tab %s: exit %d", tab, code)
		}
	}
	select {
	case m := <-got:
		t.Fatalf("unexpected message %+v", m)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestNotifyGarbageJSONStillSendsWithoutReason(t *testing.T) {
	srv, got := startServer(t)
	runNotify([]string{"needs"}, strings.NewReader("not json {{{"), envOf(srv, "5"))
	m := wait(t, got)
	if m.Tab != 5 || m.Kind != notify.KindNeeds || m.Reason != "" {
		t.Fatalf("got %+v", m)
	}
}

func TestNotifyWritesNothingToStdout(t *testing.T) {
	srv, _ := startServer(t)
	dead := envOf(srv, "1")
	srv.Close()
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old }()
	runNotify([]string{"done"}, strings.NewReader("{}"), dead)
	runNotify([]string{"bogus"}, nil, dead)
	if fi, _ := f.Stat(); fi.Size() != 0 {
		t.Fatalf("wrote %d bytes to stdout", fi.Size())
	}
}
