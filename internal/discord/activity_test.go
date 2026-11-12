package discord

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestBuildDetails(t *testing.T) {
	for _, tt := range []struct {
		project string
		show    bool
		want    string
	}{
		{"kaze-terminal", true, "Working on kaze-terminal"},
		{"", true, "In the terminal"},
		{"kaze-terminal", false, "In the terminal"},
	} {
		if got := Build(Inputs{Project: tt.project, ShowProject: tt.show}).Details; got != tt.want {
			t.Errorf("project %q, show %v: got %q, want %q", tt.project, tt.show, got, tt.want)
		}
	}
}

func TestBuildState(t *testing.T) {
	for _, tt := range []struct {
		claude, codex int
		want          string
	}{
		{0, 0, "No agents running"},
		{1, 0, "1 Claude session"},
		{3, 0, "3 Claude sessions"},
		{0, 1, "1 Codex session"},
		{0, 2, "2 Codex sessions"},
		{3, 1, "3 Claude, 1 Codex"},
	} {
		if got := Build(Inputs{Claude: tt.claude, Codex: tt.codex}).State; got != tt.want {
			t.Errorf("counts (%d,%d): got %q, want %q", tt.claude, tt.codex, got, tt.want)
		}
	}
}

func TestBuildAssets(t *testing.T) {
	start := time.Unix(1794419000, 123)
	a := Build(Inputs{Start: start, Version: "0.41"})
	if a.LargeImage != "kaze" || a.LargeText != "Kaze 0.41" || a.Start != start {
		t.Fatalf("wrong assets or start: %+v", a)
	}
}

func TestBuildClipsLongProject(t *testing.T) {
	a := Build(Inputs{Project: strings.Repeat("界", 300), ShowProject: true})
	if !utf8.ValidString(a.Details) || utf8.RuneCountInString(a.Details) > 128 || !strings.HasPrefix(a.Details, "Working on ") {
		t.Fatalf("invalid clipped details: %q", a.Details)
	}
	if got := utf8.RuneCountInString(Build(Inputs{Version: strings.Repeat("界", 300)}).LargeText); got != 128 {
		t.Fatalf("large text has %d runes, want 128", got)
	}
}

func TestActivityJSON(t *testing.T) {
	start := time.Unix(1794419000, 123)
	a := Build(Inputs{Project: `a"b\c`, ShowProject: true, Claude: 3, Codex: 1, Start: start, Version: "0.41"})
	data, err := json.Marshal(a.payload())
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"details":    `Working on a"b\c`,
		"state":      "3 Claude, 1 Codex",
		"timestamps": map[string]any{"start": float64(start.Unix())},
		"assets":     map[string]any{"large_image": "kaze", "large_text": "Kaze 0.41"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payload = %s, want %+v", data, want)
	}
}

func TestActivityTextLimits(t *testing.T) {
	long := strings.Repeat("界", 300) + "\xff"
	a := Activity{Details: long, State: long, LargeImage: long, LargeText: long}
	p := a.payload()
	assets := p["assets"].(map[string]any)
	for name, text := range map[string]string{
		"details": p["details"].(string), "state": p["state"].(string),
		"large_image": assets["large_image"].(string), "large_text": assets["large_text"].(string),
	} {
		if !utf8.ValidString(text) || utf8.RuneCountInString(text) > 128 {
			t.Errorf("%s is not valid UTF-8 within 128 runes", name)
		}
	}
	a = Build(Inputs{Project: "a\xffb", ShowProject: true})
	if !utf8.ValidString(a.Details) {
		t.Fatalf("invalid UTF-8 in built details: %q", a.Details)
	}
}
