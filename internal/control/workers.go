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

func workers(ctx context.Context, host Host, req Request, opt Options) Reply {
	h, ok := host.(WorkerHost)
	if !ok {
		return Reply{Error: &Error{CodeUnknownOp, "unknown control operation"}}
	}
	if req.Op == "spawn" {
		if req.Agent != "claude" && req.Agent != "codex" && !orchestrate.ValidTemplateName(req.Agent) {
			return Reply{Error: &Error{CodeUnknownAgent, "expected claude, codex or a valid template name"}}
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
			visible, scopeErr := VisibleTo(host.Tabs(), req.From, opt.CrossSpace)
			if scopeErr != nil {
				return Reply{Error: scopeErr}
			}
			if _, e := ValidateName(visible, "", req.Name); e != nil {
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
	tab, _, e := resolveScoped(host.Tabs(), req.Tab, req.From, opt.CrossSpace)
	if e != nil {
		return Reply{Error: e}
	}
	if tab.Role != "worker" || tab.Parent != req.From {
		return Reply{Error: &Error{CodeNotAllowed, "only your own workers can be closed"}}
	}
	return h.Kill(ctx, req.From, tab.ID)
}
