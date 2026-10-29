package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
)

var slNow = time.Unix(1738400000, 0)

const fullSample = `{"model":{"id":"claude-opus-5-5"},"context_window":{"used_percentage":34.2},
"rate_limits":{"five_hour":{"used_percentage":23.5,"resets_at":1738425600},"seven_day":{"used_percentage":41.2,"resets_at":1738857600}}}`

func TestStatuslinePrintsUsage(t *testing.T) {
	var out bytes.Buffer
	if code := runStatusline(strings.NewReader(fullSample), &out, func(string) string { return "" }, slNow); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := out.String(); got != "5h 23%  ·  wk 41%  ·  ctx 34%\n" {
		t.Fatalf("got %q", got)
	}
}

func TestStatuslineWithoutRateLimits(t *testing.T) {
	var out bytes.Buffer
	runStatusline(strings.NewReader(`{"context_window":{"used_percentage":8}}`), &out, func(string) string { return "" }, slNow)
	if got := out.String(); got != "ctx 8%\n" {
		t.Fatalf("got %q", got)
	}
}

func TestStatuslineNullContextOmitted(t *testing.T) {
	var out bytes.Buffer
	in := `{"context_window":{"used_percentage":null},"rate_limits":{"seven_day":{"used_percentage":5}}}`
	runStatusline(strings.NewReader(in), &out, func(string) string { return "" }, slNow)
	if got := out.String(); got != "wk 5%\n" {
		t.Fatalf("got %q", got)
	}
}

func TestStatuslineSendsUsageFromKazeTab(t *testing.T) {
	srv, got := startServer(t)
	var out bytes.Buffer
	runStatusline(strings.NewReader(fullSample), &out, envOf(srv, "4"), slNow)
	m := wait(t, got)
	u := m.Usage
	if m.Tab != 4 || m.Kind != notify.KindUsage || u == nil || u.FiveHour == nil || u.FiveHour.Pct != 23.5 ||
		u.FiveHour.Resets != 1738425600 || u.Week == nil || u.Week.Pct != 41.2 || u.Ctx == nil || *u.Ctx != 34.2 {
		t.Fatalf("got %+v / %+v", m, u)
	}
}

func TestStatuslineSendsEvenWithNothingToShow(t *testing.T) {
	// An empty report still tells kaze this tab's context is gone (e.g. after /clear).
	srv, got := startServer(t)
	var out bytes.Buffer
	runStatusline(strings.NewReader(`{"context_window":{"used_percentage":null}}`), &out, envOf(srv, "2"), slNow)
	m := wait(t, got)
	if m.Usage == nil || m.Usage.Ctx != nil || m.Usage.FiveHour != nil || out.Len() != 0 {
		t.Fatalf("got %+v, out %q", m.Usage, out.String())
	}
}

func TestStatuslineGarbageIsQuiet(t *testing.T) {
	srv, got := startServer(t)
	var out bytes.Buffer
	if code := runStatusline(strings.NewReader("not json"), &out, envOf(srv, "1"), slNow); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out.Len() != 0 {
		t.Fatalf("printed %q", out.String())
	}
	select {
	case m := <-got:
		t.Fatalf("sent %+v", m)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestStatuslineNilStdin(t *testing.T) {
	var out bytes.Buffer
	if code := runStatusline(nil, &out, func(string) string { return "" }, slNow); code != 0 || out.Len() != 0 {
		t.Fatalf("exit %d out %q", code, out.String())
	}
}
