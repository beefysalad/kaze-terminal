package control

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeHost struct {
	tabs                               []TabInfo
	text                               string
	err                                error
	tabCalls, screenCalls, submitCalls int
	id                                 string
	lines                              int
	input                              []byte
}

func (h *fakeHost) Tabs() []TabInfo { h.tabCalls++; return h.tabs }
func (h *fakeHost) Screen(id string, lines int) (string, error) {
	h.screenCalls++
	h.id, h.lines = id, lines
	return h.text, h.err
}
func (h *fakeHost) Submit(id string, input []byte) error {
	h.submitCalls++
	h.id, h.input = id, input
	return h.err
}

func TestHandleUnknownOp(t *testing.T) {
	for _, op := range []string{"", "bogus"} {
		h := &fakeHost{}
		reply := Handle(h, Request{Op: op})
		if reply.OK || reply.Error == nil || reply.Error.Code != CodeUnknownOp {
			t.Fatalf("%q: reply = %#v", op, reply)
		}
		if h.tabCalls+h.screenCalls+h.submitCalls != 0 {
			t.Fatal("unsupported operation called Host")
		}
	}
}

func TestHandleList(t *testing.T) {
	for _, tabs := range [][]TabInfo{nil, {{ID: "t3", Name: "review", Space: "kaze", Folder: "/repo", Agent: "codex", Status: "idle", Workspace: true}}} {
		h := &fakeHost{tabs: tabs}
		reply := Handle(h, Request{Op: "ls"})
		if !reply.OK || reply.Error != nil || !reflect.DeepEqual(reply.Tabs, tabs) {
			t.Fatalf("reply = %#v; want tabs %#v", reply, tabs)
		}
		if h.tabCalls != 1 || h.screenCalls+h.submitCalls != 0 {
			t.Fatalf("unexpected Host calls: %#v", h)
		}
	}
}

func TestHandleRead(t *testing.T) {
	// Screen conversion is the Host's job. These already-converted fixtures
	// must survive the handler unchanged, including blank lines and indentation.
	for _, tc := range []struct {
		name, text string
		lines      int
	}{
		{"visible", "prompt\n  answer\n", 0},
		{"wrapped rows joined and tails trimmed", "one long wrapped line\nnext", 0},
		{"scrollback plus screen", "history\n\nvisible", 2},
		{"alternate screen only", "editor\n  buffer", 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &fakeHost{tabs: []TabInfo{{ID: "t3", Name: "review"}}, text: tc.text}
			reply := Handle(h, Request{Op: "read", Tab: "review", Lines: tc.lines})
			if !reply.OK || reply.Error != nil || reply.Text != tc.text {
				t.Fatalf("reply = %#v", reply)
			}
			if h.tabCalls != 1 || h.screenCalls != 1 || h.submitCalls != 0 || h.id != "t3" || h.lines != tc.lines {
				t.Fatalf("unexpected Host calls: %#v", h)
			}
		})
	}
}

func TestHandleReadLimits(t *testing.T) {
	// The fake never allocates in proportion to Lines, even before clamping.
	for _, tc := range []struct{ requested, want int }{{0, 0}, {2000, 2000}, {2001, 2000}, {int(^uint(0) >> 1), 2000}} {
		h := &fakeHost{tabs: []TabInfo{{ID: "t3"}}}
		reply := Handle(h, Request{Op: "read", Tab: "t3", Lines: tc.requested})
		if !reply.OK || h.screenCalls != 1 || h.lines != tc.want {
			t.Fatalf("lines %d: reply %#v, screen calls %d, lines %d", tc.requested, reply, h.screenCalls, h.lines)
		}
	}
	h := &fakeHost{tabs: []TabInfo{{ID: "t3"}}}
	reply := Handle(h, Request{Op: "read", Tab: "t3", Lines: -1})
	if reply.OK || reply.Error == nil || reply.Error.Code != CodeInvalidRequest || h.screenCalls+h.submitCalls != 0 {
		t.Fatalf("negative lines: reply %#v, Host %#v", reply, h)
	}
}

func TestHandleReadErrors(t *testing.T) {
	for _, tc := range []struct{ address, code string }{
		{"", CodeInvalidRequest}, {"missing", CodeUnknownTab}, {"review", CodeAmbiguousTab}, {"t1", CodeHostError},
	} {
		h := &fakeHost{tabs: []TabInfo{{ID: "t1", Name: "review"}, {ID: "t2", Name: "review"}}, text: "partial", err: errors.New("host failure")}
		reply := Handle(h, Request{Op: "read", Tab: tc.address})
		if reply.OK || reply.Error == nil || reply.Error.Code != tc.code || reply.Text != "" {
			t.Fatalf("%q: reply = %#v", tc.address, reply)
		}
		wantCalls := 0
		if tc.address == "t1" {
			wantCalls = 1
		}
		if h.screenCalls != wantCalls || h.submitCalls != 0 {
			t.Fatalf("%q: unexpected Host calls: %#v", tc.address, h)
		}
	}
}

func TestHandleSend(t *testing.T) {
	for _, text := range []string{"", "hello\n  world", "\u65e5\u672c\u8a9e", strings.Repeat("x", 32<<10)} {
		h := &fakeHost{tabs: []TabInfo{{ID: "t3", Name: "review"}}}
		reply := Handle(h, Request{Op: "send", Tab: "review", Text: text})
		if !reply.OK || reply.Error != nil {
			t.Fatalf("%d bytes: reply = %#v", len(text), reply)
		}
		if h.tabCalls != 1 || h.submitCalls != 1 || h.screenCalls != 0 || h.id != "t3" || string(h.input) != text {
			t.Fatalf("%d bytes: submission count %d, id %q, bytes %d", len(text), h.submitCalls, h.id, len(h.input))
		}
	}
}

func TestHandleSendCap(t *testing.T) {
	// Both allocations are capped at 32 KiB plus at most 3 bytes. The second
	// case is under the cap in runes but over it in UTF-8 bytes.
	for _, text := range []string{strings.Repeat("x", (32<<10)+1), strings.Repeat("\u754c", (32<<10)/3+1)} {
		h := &fakeHost{tabs: []TabInfo{{ID: "t3"}}}
		reply := Handle(h, Request{Op: "send", Tab: "t3", Text: text})
		if reply.OK || reply.Error == nil || reply.Error.Code != CodeTextTooLarge {
			t.Fatalf("%d bytes: reply = %#v", len(text), reply)
		}
		if h.tabCalls+h.screenCalls+h.submitCalls != 0 {
			t.Fatal("oversize text reached Host")
		}
	}
}

func TestHandleSendErrors(t *testing.T) {
	for _, tc := range []struct{ address, code string }{
		{"", CodeInvalidRequest}, {"missing", CodeUnknownTab}, {"review", CodeAmbiguousTab}, {"t1", CodeHostError},
	} {
		t.Run(tc.code, func(t *testing.T) {
			h := &fakeHost{tabs: []TabInfo{{ID: "t1", Name: "review"}, {ID: "t2", Name: "review"}}, err: errors.New("host failure")}
			reply := Handle(h, Request{Op: "send", Tab: tc.address, Text: "hello"})
			if reply.OK || reply.Error == nil || reply.Error.Code != tc.code {
				t.Fatalf("reply = %#v", reply)
			}
			wantCalls := 0
			if tc.address == "t1" {
				wantCalls = 1
			}
			if h.submitCalls != wantCalls || h.screenCalls != 0 {
				t.Fatalf("unexpected Host calls: %#v", h)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tabs := []TabInfo{
		{ID: "t1", Name: "review"},
		{ID: "t3", Name: "build"},
		{ID: "t4", Name: "review"},
		{ID: "t5", Name: "t3"},
		{ID: "t6"},
	}
	for _, tc := range []struct{ address, id, code string }{
		{"t1", "t1", ""},
		{"t3", "t3", ""}, // ids win over a colliding name
		{"build", "t3", ""},
		{"review", "", CodeAmbiguousTab},
		{"missing", "", CodeUnknownTab},
		{"Build", "", CodeUnknownTab},
		{" build ", "", CodeUnknownTab},
		{"", "", CodeInvalidRequest},
	} {
		t.Run(tc.address, func(t *testing.T) {
			got, err := Resolve(tabs, tc.address)
			if tc.code != "" {
				if err == nil || err.Code != tc.code || got.ID != "" {
					t.Fatalf("Resolve = %#v, %v; want %s", got, err, tc.code)
				}
				return
			}
			if err != nil || got.ID != tc.id {
				t.Fatalf("Resolve = %#v, %v; want %s", got, err, tc.id)
			}
		})
	}
	if _, err := Resolve(nil, "t1"); err == nil || err.Code != CodeUnknownTab {
		t.Fatalf("empty snapshot: %v", err)
	}
}

func (h *fakeHost) Keys(id string, keys []string) error {
	h.submitCalls++
	h.id, h.input = id, []byte(strings.Join(keys, " "))
	return h.err
}
func (h *fakeHost) Interrupt(id string, ctrlC bool) error {
	h.submitCalls++
	h.id = id
	if ctrlC {
		h.input = []byte("ctrl+c")
	}
	return h.err
}

func TestHandleKeysAndInterrupt(t *testing.T) {
	for _, op := range []string{"keys", "interrupt"} {
		for _, addr := range []string{"review", "missing", ""} {
			for _, hostErr := range []error{nil, errors.New("closed")} {
				h := &fakeHost{tabs: []TabInfo{{ID: "t1", Name: "review"}}, err: hostErr}
				r := Handle(h, Request{Op: op, Tab: addr, Args: []string{"EnTeR", "CTRL+C"}, CtrlC: true})
				code := ""
				switch {
				case addr == "":
					code = CodeInvalidRequest
				case addr == "missing":
					code = CodeUnknownTab
				case hostErr != nil:
					code = CodeHostError
				}
				if code != "" {
					if r.OK || r.Error == nil || r.Error.Code != code {
						t.Fatalf("%s %q: %+v", op, addr, r)
					}
				} else if !r.OK || r.Error != nil {
					t.Fatalf("%s: %+v", op, r)
				}
				want := 0
				if addr == "review" {
					want = 1
				}
				if h.submitCalls != want || h.screenCalls != 0 {
					t.Fatalf("unexpected calls: %+v", h)
				}
				if want == 1 && (h.id != "t1" || (op == "keys" && string(h.input) != "EnTeR CTRL+C") || (op == "interrupt" && string(h.input) != "ctrl+c")) {
					t.Fatalf("delegation: %+v", h)
				}
			}
		}
	}
}

func TestHandleKeysValidation(t *testing.T) {
	for _, keys := range [][]string{nil, {}, {"enter", "bogus"}, {""}, {" enter"}, strings.Fields(strings.Repeat("enter ", 65))} {
		h := &fakeHost{tabs: []TabInfo{{ID: "t1"}}}
		r := Handle(h, Request{Op: "keys", Tab: "t1", Args: keys})
		code := CodeInvalidRequest
		if len(keys) > 0 && len(keys) <= 64 {
			code = CodeUnknownKey
		}
		if r.OK || r.Error == nil || r.Error.Code != code || h.tabCalls+h.submitCalls != 0 {
			t.Fatalf("keys %q: %+v host %+v", keys, r, h)
		}
	}
	keys := strings.Fields(strings.Repeat("enter ", 64))
	h := &fakeHost{tabs: []TabInfo{{ID: "t1"}}}
	if r := Handle(h, Request{Op: "keys", Tab: "t1", Args: keys}); !r.OK {
		t.Fatalf("64 keys: %+v", r)
	}
}
