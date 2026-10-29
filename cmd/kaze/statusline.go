package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
	"github.com/patrick-gogo/kaze-terminal/internal/usage"
)

// statuslineTotal bounds a run: Claude Code reruns the command on every update
// and cancels one still running, so it has to be quick.
const statuslineTotal = 300 * time.Millisecond

type statusInput struct {
	Context struct {
		Used *float64 `json:"used_percentage"`
	} `json:"context_window"`
	Limits struct {
		FiveHour *limitWindow `json:"five_hour"`
		Week     *limitWindow `json:"seven_day"`
	} `json:"rate_limits"`
}

type limitWindow struct {
	Used   *float64 `json:"used_percentage"`
	Resets int64    `json:"resets_at"`
}

func (w *limitWindow) window() *notify.Window {
	if w == nil || w.Used == nil {
		return nil
	}
	return &notify.Window{Pct: *w.Used, Resets: w.Resets}
}

// runStatusline is `kaze statusline`, Claude Code's status line command. It
// prints the usage line for Claude's own status row and, inside a kaze tab,
// sends the numbers to kaze. It always returns 0.
func runStatusline(stdin io.Reader, stdout io.Writer, env func(string) string, now time.Time) (code int) {
	defer func() {
		if recover() != nil {
			code = 0
		}
	}()
	if stdin == nil {
		return 0
	}
	start := time.Now()
	var in statusInput
	raw := bytes.TrimPrefix(readStdinLimited(stdin, statuslineTotal), []byte("\xef\xbb\xbf")) // PowerShell may add a BOM
	if json.Unmarshal(raw, &in) != nil {
		return 0
	}
	u := notify.Usage{FiveHour: in.Limits.FiveHour.window(), Week: in.Limits.Week.window(), Ctx: in.Context.Used}
	if line := usage.Line(u, now); line != "" {
		fmt.Fprintln(stdout, line)
	}
	addr, token, tab, ok := kazeTab(env)
	if !ok {
		return 0
	}
	if left := statuslineTotal - time.Since(start); left > 0 {
		_ = notify.Send(addr, token, notify.Msg{Tab: tab, Kind: notify.KindUsage, Usage: &u}, left)
	}
	return 0
}
