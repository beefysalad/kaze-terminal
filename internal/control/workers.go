package control

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/beefysalad/kaze-terminal/internal/orchestrate"
)

// WorkerHost optionally supports UI-queued worker management. Existing hosts
// keep their previous unknown_op behavior when they do not implement it.
type WorkerHost interface {
	Spawn(context.Context, Request) Reply
	Kill(context.Context, string, string) Reply
}

func workers(ctx context.Context, host Host, req Request) Reply {
	h, ok := host.(WorkerHost)
	if !ok {
		return Reply{Error: &Error{CodeUnknownOp, "unknown control operation"}}
	}
	if req.Op == "spawn" {
		if req.Agent != "claude" && req.Agent != "codex" {
			return Reply{Error: &Error{CodeUnknownAgent, "agent must be claude or codex"}}
		}
		if len(req.Task) > orchestrate.MaxTaskBytes {
			return Reply{Error: &Error{CodeTextTooLarge, "task exceeds 16 KiB"}}
		}
		if strings.TrimSpace(req.Task) == "" || !utf8.ValidString(req.Task) || strings.ContainsRune(req.Task, 0) {
			return Reply{Error: &Error{CodeInvalidRequest, "task must be nonempty UTF-8 text without NUL"}}
		}
		if req.Mode != "" && req.Mode != "edit" && req.Mode != "read-only" {
			return Reply{Error: &Error{CodeInvalidRequest, "mode must be edit or read-only"}}
		}
		if req.Name != "" {
			if _, e := ValidateName(host.Tabs(), "", req.Name); e != nil {
				return Reply{Error: e}
			}
		}

		if len(req.Labels) > 8 {
			return Reply{Error: &Error{CodeInvalidRequest, "at most 8 labels"}}
		}
		for k, v := range req.Labels {
			if e := ValidateLabel(k, v); e != nil {
				return Reply{Error: e}
			}
		}
		return h.Spawn(ctx, req)
	}
	tab, e := Resolve(host.Tabs(), req.Tab)
	if e != nil {
		return Reply{Error: e}
	}
	if tab.Role != "worker" || tab.Parent != req.From {
		return Reply{Error: &Error{CodeNotAllowed, "only your own workers can be closed"}}
	}
	return h.Kill(ctx, req.From, tab.ID)
}
