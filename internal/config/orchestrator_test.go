package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestOrchestratorConfig(t *testing.T) {
	defaults := Orchestrator{Confirm: "first", NoteQuiet: 5, MaxDepth: 1}
	if got := Default().Orchestrator; got != defaults {
		t.Fatalf("defaults: %+v", got)
	}
	for _, tc := range []struct {
		name, source string
		want         Orchestrator
		warning      string
	}{
		{"absent", "", defaults, ""},
		{"enabled", "[orchestrator]\nenabled = true", Orchestrator{true, "first", 5, 1}, ""},
		{"disabled", "[orchestrator]\nenabled = false", defaults, ""},
		{"first", "[orchestrator]\nconfirm = 'first'", defaults, ""},
		{"always", "[orchestrator]\nconfirm = 'always'", Orchestrator{false, "always", 5, 1}, ""},
		{"never", "[orchestrator]\nconfirm = 'never'", Orchestrator{false, "never", 5, 1}, ""},
		{"quiet minimum", "[orchestrator]\nnote_quiet = 1", Orchestrator{false, "first", 1, 1}, ""},
		{"quiet maximum", "[orchestrator]\nnote_quiet = 60", Orchestrator{false, "first", 60, 1}, ""},
		{"depth", "[orchestrator]\nmax_depth = 1", defaults, ""},
		{"bad table", "orchestrator = false", defaults, "orchestrator must be a table"},
		{"bad enabled", "[orchestrator]\nenabled = 'yes'", defaults, "orchestrator.enabled"},
		{"unknown confirm", "[orchestrator]\nconfirm = 'sometimes'", defaults, "orchestrator.confirm"},
		{"bad confirm type", "[orchestrator]\nconfirm = true", defaults, "orchestrator.confirm"},
		{"quiet zero", "[orchestrator]\nnote_quiet = 0", defaults, "orchestrator.note_quiet"},
		{"quiet negative", "[orchestrator]\nnote_quiet = -1", defaults, "orchestrator.note_quiet"},
		{"quiet excessive", "[orchestrator]\nnote_quiet = 61", defaults, "orchestrator.note_quiet"},
		{"quiet wrong type", "[orchestrator]\nnote_quiet = 5.0", defaults, "orchestrator.note_quiet"},
		{"depth zero", "[orchestrator]\nmax_depth = 0", defaults, "orchestrator.max_depth"},
		{"depth negative", "[orchestrator]\nmax_depth = -1", defaults, "orchestrator.max_depth"},
		{"depth excessive", "[orchestrator]\nmax_depth = 2", defaults, "orchestrator.max_depth"},
		{"depth wrong type", "[orchestrator]\nmax_depth = '1'", defaults, "orchestrator.max_depth"},
		{"unknown key", "[orchestrator]\nother = true", defaults, "orchestrator.other"},
		{"control dependency", "[orchestrator]\nenabled = true\n[control]\nenabled = false", Orchestrator{true, "first", 5, 1}, "[orchestrator] enabled needs [control] enabled = true"},
		{"both disabled", "[orchestrator]\nenabled = false\n[control]\nenabled = false", defaults, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, warnings, err := Load(write(t, tc.source))
			if err != nil || got.Orchestrator != tc.want {
				t.Fatalf("got %+v, err %v; want %+v", got.Orchestrator, err, tc.want)
			}
			if tc.warning == "" {
				if len(warnings) != 0 {
					t.Fatal(warnings)
				}
			} else if len(warnings) != 1 || !strings.Contains(warnings[0], tc.warning) {
				t.Fatalf("warnings %v; want %q", warnings, tc.warning)
			}
		})
	}
}

func TestOrchestratorNoteQuietRange(t *testing.T) {
	for seconds := 1; seconds <= 60; seconds++ {
		got, warnings, err := Load(write(t, fmt.Sprintf("[orchestrator]\nnote_quiet = %d", seconds)))
		if err != nil || len(warnings) != 0 || got.Orchestrator.NoteQuiet != seconds {
			t.Fatalf("%ds: got %+v, warnings %v, err %v", seconds, got.Orchestrator, warnings, err)
		}
	}
}
