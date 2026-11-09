package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func systemNotifications(t *testing.T, c Config) bool {
	t.Helper()
	v := reflect.ValueOf(c).FieldByName("SystemNotifications")
	if !v.IsValid() || v.Kind() != reflect.Bool {
		t.Fatal("missing system_notifications boolean")
	}
	return v.Bool()
}

func TestSystemNotificationsConfiguration(t *testing.T) {
	if !systemNotifications(t, Default()) {
		t.Fatal("system notifications must default on")
	}
	for _, tc := range []struct {
		body     string
		want     bool
		warnings int
	}{
		{"", true, 0}, {"system_notifications = false\n", false, 0},
		{"system_notifications = true\n", true, 0}, {"system_notifications = 3\n", true, 1},
	} {
		c, ws, err := Load(write(t, tc.body))
		if err != nil || systemNotifications(t, c) != tc.want || len(ws) != tc.warnings {
			t.Fatalf("%q: config=%+v warnings=%v err=%v", tc.body, c, ws, err)
		}
	}
}

func TestSystemNotificationsSetPreservesComments(t *testing.T) {
	p := write(t, "# keep\nsystem_notifications = true # alerts\n[colors]\nfg = '#ffffff'\n")
	if err := Set(p, "system_notifications", false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "system_notifications = false # alerts") || !strings.Contains(string(b), "# keep\n") || !strings.Contains(string(b), "fg = '#ffffff'") {
		t.Fatal(string(b))
	}
	c, ws, err := Load(p)
	if err != nil || len(ws) != 0 || systemNotifications(t, c) {
		t.Fatalf("round trip: %+v %v %v", c, ws, err)
	}
}
