package control

import (
	"context"
	"testing"
)

type templatesHost struct{ workerHost }

func (h *templatesHost) Templates(context.Context) Reply {
	return Reply{OK: true, Text: "review  codex  Review changes"}
}
func TestTemplatesControlDispatch(t *testing.T) {
	h := &templatesHost{}
	r := Handle(h, Request{Op: "templates"})
	if !r.OK || r.Text != "review  codex  Review changes" {
		t.Fatal(r)
	}
	r = Handle(h, Request{Op: "spawn", From: "t1", CallerKey: h.OwnerKey("t1"), Agent: "review", Task: "Review"})
	if !r.OK || h.calls != 1 {
		t.Fatal(r, h.calls)
	}
}
