package notify

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestControlConsentSpawnBudgets(t *testing.T) {
	for _, waitStart := range []bool{false, true} {
		t.Run(map[bool]string{false: "spawn", true: "wait_start"}[waitStart], func(t *testing.T) {
			s := controlServer(t, nil)
			s.SetControlContext(func(ctx context.Context, _ json.RawMessage) any {
				deadline, ok := ctx.Deadline()
				remaining := time.Until(deadline)
				budget := 65 * time.Second
				if waitStart {
					budget = 100 * time.Second
				}
				if !ok || remaining < budget-time.Second || remaining > budget {
					t.Errorf("deadline %v, want %v", remaining, budget)
				}
				if len(s.waitSem) != 1 || len(s.controlSem) != 0 {
					t.Errorf("admission wait=%d control=%d", len(s.waitSem), len(s.controlSem))
				}
				return map[string]any{"ok": true}
			})
			wire := `{"op":"spawn","token":"test-control-token","wait_start":false}`
			if waitStart {
				wire = `{"op":"spawn","token":"test-control-token","wait_start":true}`
			}
			if got := controlExchange(t, s, wire); got["ok"] != true {
				t.Fatal(got)
			}
		})
	}
}
