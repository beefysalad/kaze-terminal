package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// discordFlag reads a field by name so the RED run compiles before the field exists.
func discordFlag(t *testing.T, c Config, field string) bool {
	t.Helper()
	v := reflect.ValueOf(c).FieldByName(field)
	if !v.IsValid() || v.Kind() != reflect.Bool {
		t.Fatalf("missing %s boolean", field)
	}
	return v.Bool()
}

func TestDiscordPresenceConfiguration(t *testing.T) {
	for _, key := range []struct{ toml, field string }{
		{"discord_presence", "DiscordPresence"},
		{"discord_show_project", "DiscordShowProject"},
	} {
		if !discordFlag(t, Default(), key.field) {
			t.Fatalf("%s must default on", key.toml)
		}
		for _, tc := range []struct {
			body     string
			want     bool
			warnings int
		}{
			{"", true, 0}, {key.toml + " = false\n", false, 0},
			{key.toml + " = true\n", true, 0}, {key.toml + " = 3\n", true, 1},
		} {
			c, ws, err := Load(write(t, tc.body))
			if err != nil || discordFlag(t, c, key.field) != tc.want || len(ws) != tc.warnings {
				t.Fatalf("%q: config=%+v warnings=%v err=%v", tc.body, c, ws, err)
			}
		}
	}
}

func TestDiscordPresenceSetPreservesComments(t *testing.T) {
	p := write(t, "# keep\ndiscord_presence = true # card\n[colors]\nfg = '#ffffff'\n")
	if err := Set(p, "discord_presence", false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "discord_presence = false # card") || !strings.Contains(string(b), "# keep\n") || !strings.Contains(string(b), "fg = '#ffffff'") {
		t.Fatal(string(b))
	}
	c, ws, err := Load(p)
	if err != nil || len(ws) != 0 || discordFlag(t, c, "DiscordPresence") {
		t.Fatalf("round trip: %+v %v %v", c, ws, err)
	}
}
