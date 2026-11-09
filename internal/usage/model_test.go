package usage

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestModelName(t *testing.T) {
	for _, c := range []struct{ id, display, want string }{
		{"claude-opus-5-5", "", "Opus 5.5"},
		{"claude-fable-5-1", "ignored", "Fable 5.1"},
		{"claude-haiku-5-5-20260101", "", "Haiku 5.5"},
		{"claude-opus-5-5[1m]", "", "Opus 5.5"},
		{"claude-3-5-sonnet-20241022", "", "Sonnet 3.5"},
		{"us.anthropic.claude-opus-4-1-20250805-v1:0", "", "Opus 4.1"},
		{"claude-sonnet-4-5-20250929", "", "Sonnet 4.5"},
		{"us.anthropic.claude-sonnet-4-5-20250929-v1:0", "", "Sonnet 4.5"},
		{"arn:aws:bedrock:us-east-1:1:inference-profile/us.anthropic.claude-opus-4-1-20250805-v1:0", "", "Opus 4.1"},
		{"claude-sonnet-4-5@20250929", "", "Sonnet 4.5"},
		{"claude-opus-4-1@20250805", "", "Opus 4.1"},
		{"claude-3-5-sonnet-latest", "", "Sonnet 3.5"},
		{"claude-sonnet-4-5-thinking", "", "Sonnet 4.5"},
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

func TestModelNameCapIsInBytesAndKeepsRunesWhole(t *testing.T) {
	got := ModelName("", strings.Repeat("語", 30))
	if len(got) > maxModelName || got == "" || !utf8.ValidString(got) {
		t.Fatalf("len=%d valid=%v", len(got), utf8.ValidString(got))
	}
}

func TestModelFamily(t *testing.T) {
	for in, want := range map[string]string{"Opus 5.5": "Opus", "gpt-5": "gpt-5", "": ""} {
		if got := ModelFamily(in); got != want {
			t.Errorf("%q=%q", in, got)
		}
	}
}
