package usage

import (
	"strings"
	"testing"
)

func TestModelName(t *testing.T) {
	for _, c := range []struct{ id, display, want string }{
		{"claude-opus-5-5", "", "Opus 5.5"},
		{"claude-fable-5-1", "ignored", "Fable 5.1"},
		{"claude-haiku-5-5-20260101", "", "Haiku 5.5"},
		{"claude-opus-5-5[1m]", "", "Opus 5.5"},
		{"claude-3-5-sonnet-20241022", "", "Sonnet 3.5"},
		{"us.anthropic.claude-opus-4-1-20250805-v1:0", "", "Opus 4.1"},
		{"some-custom-model", "Custom One", "Custom One"},
		{"some-custom-model", "", "some-custom-model"},
		{"", "", ""},
	} {
		if got := ModelName(c.id, c.display); got != c.want {
			t.Errorf("ModelName(%q, %q)=%q want %q", c.id, c.display, got, c.want)
		}
	}
}

func TestModelNameIsBounded(t *testing.T) {
	if got := ModelName(strings.Repeat("x", 500), ""); got == "" || len(got) > maxModelName {
		t.Fatalf("len=%d", len(got))
	}
}

func TestModelFamily(t *testing.T) {
	for in, want := range map[string]string{"Opus 5.5": "Opus", "gpt-5": "gpt-5", "": ""} {
		if got := ModelFamily(in); got != want {
			t.Errorf("%q=%q", in, got)
		}
	}
}
