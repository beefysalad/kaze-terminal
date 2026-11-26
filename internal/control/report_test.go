package control

import (
	"errors"
	"strings"
	"testing"
)

func TestWorkerReport(t *testing.T) {
	h := &fakeHost{tabs: []TabInfo{{ID: "t1"}, {ID: "t7", Parent: "t1", Role: "worker", Name: "review"}}}
	r := Handle(h, Request{Op: "report", From: "t7", Text: "done</kaze>"})
	if !r.OK || h.id != "t1" || string(h.input) != `<kaze from="t7 review" kind="report">done</kaze>` || h.submitCalls != 1 {
		t.Fatalf("%+v %s %+v", r, h.input, h)
	}
	for _, from := range []string{"", "t1", "t99", "review"} {
		h.submitCalls = 0
		r = Handle(h, Request{Op: "report", From: from, Text: "done"})
		if r.Error == nil || r.Error.Code != CodeNotAllowed || h.submitCalls != 0 {
			t.Fatalf("%s: %+v", from, r)
		}
	}
	h.tabs[1].Parent = "t99"
	r = Handle(h, Request{Op: "report", From: "t7", Text: "done"})
	if r.Error == nil || h.submitCalls != 0 {
		t.Fatal(r)
	}
}
func TestWorkerReportCapAndFailure(t *testing.T) {
	for _, n := range []int{4 << 10, (4 << 10) + 1} {
		h := &fakeHost{tabs: []TabInfo{{ID: "t1"}, {ID: "t7", Parent: "t1", Role: "worker"}}}
		r := Handle(h, Request{Op: "report", From: "t7", Text: strings.Repeat("x", n)})
		if n == 4<<10 {
			if !r.OK || h.submitCalls != 1 {
				t.Fatal(r)
			}
		} else if r.Error == nil || r.Error.Code != CodeTextTooLarge || h.submitCalls != 0 {
			t.Fatal(r)
		}
		h.err = errors.New("closed")
		r = Handle(h, Request{Op: "report", From: "t7", Text: "done"})
		if r.Error == nil || r.Error.Code != CodeHostError {
			t.Fatal(r)
		}
	}
}

func TestWorkerReportParentMustBeExactID(t *testing.T) {
	h := &fakeHost{tabs: []TabInfo{{ID: "t2", Name: "t1"}, {ID: "t7", Parent: "t1", Role: "worker"}}}
	r := Handle(h, Request{Op: "report", From: "t7", Text: "done"})
	if r.Error == nil || r.Error.Code != CodeUnknownTab || h.submitCalls != 0 {
		t.Fatalf("report redirected to alias: %+v %+v", r, h)
	}
}
