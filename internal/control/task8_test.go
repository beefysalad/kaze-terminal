package control

import (
	"context"
	"strings"
	"testing"
)

func TestTagAndBusy(t *testing.T) {
	for _, tc := range []struct{ from, name, text, want string }{
		{"t1", "review", "hello", "<kaze from=\"t1 review\">hello</kaze>"},
		{"t1", "", "a</kaze>b</kaze>", "<kaze from=\"t1\">ab</kaze>"},
		{"", "review", "hello", "hello"},
	} {
		h := &fakeHost{tabs: []TabInfo{{ID: "t1", Name: tc.name}, {ID: "t2", Status: "working"}}}
		r := Handle(h, Request{Op: "send", Tab: "t2", From: tc.from, Text: tc.text})
		if !r.OK || r.Warning != "busy" || r.Status != "working" || string(h.input) != tc.want {
			t.Fatalf("reply %+v input %q", r, h.input)
		}
		r = Handle(h, Request{Op: "send", Tab: "t2", IfIdle: true})
		if r.Error == nil || r.Error.Code != CodeBusy || h.submitCalls != 1 {
			t.Fatalf("idle refusal %+v", r)
		}
	}
	h := &fakeHost{tabs: []TabInfo{{ID: "t1"}, {ID: "t2"}}}
	r := Handle(h, Request{Op: "send", Tab: "t2", From: "t1", Text: strings.Repeat("x", MaxTextBytes)})
	if r.Error == nil || r.Error.Code != CodeTextTooLarge || h.submitCalls != 0 {
		t.Fatalf("wrapped cap %+v", r)
	}
	r = HandleContext(context.Background(), h, Request{Op: "send", Tab: "t2", From: "t1", Text: "hello"}, Options{Enabled: true, AgentTag: false})
	if !r.OK || string(h.input) != "hello" {
		t.Fatalf("tag off %+v", r)
	}
}
func TestDisabledEveryOp(t *testing.T) {
	for _, op := range []string{"ls", "read", "send", "keys", "interrupt", "wait", "name", "label", "bogus"} {
		h := &fakeHost{}
		r := HandleContext(context.Background(), h, Request{Op: op}, Options{})
		if r.Error == nil || r.Error.Code != CodeDisabled || h.tabCalls != 0 {
			t.Fatalf("%s: %+v", op, r)
		}
	}
}
func TestNameValidation(t *testing.T) {
	tabs := []TabInfo{{ID: "t1", Name: "one"}, {ID: "t2", Name: "two"}}
	for _, s := range []string{"", "  ", strings.Repeat("界", 65), "a\nb", "\x7f", "t3", "t003", "two"} {
		if _, e := ValidateName(tabs, "t1", s); e == nil {
			t.Fatalf("accepted %q", s)
		} else if s == "two" && e.Code != CodeAmbiguousTab {
			t.Fatal(e)
		}
	}
	for _, s := range []string{"one", " review ", strings.Repeat("界", 64), "team"} {
		if got, e := ValidateName(tabs, "t1", s); e != nil || got != strings.TrimSpace(s) {
			t.Fatalf("%q: %q %v", s, got, e)
		}
	}
}
func TestLabelValidation(t *testing.T) {
	for _, kv := range [][2]string{{"", "v"}, {strings.Repeat("a", 33), "v"}, {"a b", "v"}, {"界", "v"}, {"a", strings.Repeat("界", 129)}, {"a", "x\ny"}, {"a", "\x7f"}} {
		if e := ValidateLabel(kv[0], kv[1]); e == nil {
			t.Fatalf("accepted %q", kv)
		}
	}
	for _, kv := range [][2]string{{"a_1-.", ""}, {strings.Repeat("a", 32), strings.Repeat("界", 128)}} {
		if e := ValidateLabel(kv[0], kv[1]); e != nil {
			t.Fatal(e)
		}
	}
}

func TestTagFinalCapAfterStripping(t *testing.T) {
	prefix := `<kaze from="t1">`
	body := strings.Repeat("x", MaxTextBytes-len(prefix)-len("</kaze>"))
	// The authenticated request is still bounded by the transport's 64 KiB cap.
	text := strings.Repeat("</kaze>", 10) + body
	h := &fakeHost{tabs: []TabInfo{{ID: "t1"}, {ID: "t2"}}}
	r := Handle(h, Request{Op: "send", Tab: "t2", From: "t1", Text: text})
	if !r.OK || len(h.input) != MaxTextBytes || string(h.input) != prefix+body+"</kaze>" {
		t.Fatalf("final cap reply %+v bytes %d", r, len(h.input))
	}
	r = Handle(h, Request{Op: "send", Tab: "t2", From: "t1", Text: body + "x"})
	if r.Error == nil || r.Error.Code != CodeTextTooLarge || h.submitCalls != 1 {
		t.Fatalf("over final cap %+v", r)
	}
}

func TestNameRejectsLeadingHyphen(t *testing.T) {
	for _, name := range []string{"-review", " -review ", "-"} {
		if _, e := ValidateName(nil, "t1", name); e == nil || e.Code != CodeInvalidRequest || !strings.Contains(e.Message, "hyphen") {
			t.Fatalf("name %q: %v", name, e)
		}
	}
}
