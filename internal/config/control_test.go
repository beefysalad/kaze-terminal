package config

import "testing"

func TestControlConfig(t *testing.T) {
	for _, tc := range []struct {
		source       string
		tag, enabled bool
		warnings     int
	}{
		{"", true, true, 0}, {"font_size = 16", true, true, 0},
		{"agent_tag = false\n[control]\nenabled = false", false, false, 0},
		{"agent_tag = true\n[control]\nenabled = true", true, true, 0},
		{"agent_tag = 'bad'\n[control]\nenabled = 'bad'\nother = true", true, true, 3},
		{"control = false", true, true, 1}, {"agent_tags = false", true, true, 1},
	} {
		c, ws, e := Load(write(t, tc.source))
		if e != nil || c.AgentTag != tc.tag || c.Control.Enabled != tc.enabled || len(ws) != tc.warnings {
			t.Fatalf("%q: %+v warnings %v err %v", tc.source, c, ws, e)
		}
	}
	if c := Default(); !c.AgentTag || !c.Control.Enabled {
		t.Fatal("defaults off")
	}
}
