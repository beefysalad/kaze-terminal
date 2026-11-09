package codexlaunch

import (
	"reflect"
	"testing"
)

func TestIsolationArguments(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		isolated bool
	}{
		{"plain", nil, true}, {"prompt", []string{"fix the tests"}, true}, {"ambiguous prompt", []string{"hello"}, false},
		{"resume", []string{"resume", "abc"}, true}, {"fork", []string{"fork", "--last"}, true},
		{"options", []string{"-m", "model", "--cd", "dir with spaces"}, true},
		{"remote", []string{"--remote", "ws://local"}, false}, {"remote equals", []string{"resume", "--remote=ws://local"}, false},
		{"existing flag", []string{"--no-daemon", "resume", "abc"}, false},
		{"help", []string{"resume", "--help"}, false}, {"version", []string{"--version"}, false},
		{"daemon admin", []string{"app-server", "daemon", "stop"}, false}, {"queue", []string{"queue", "hello"}, false},
		{"unknown command", []string{"future-command"}, false}, {"unknown option", []string{"--future-option", "value"}, false},
		{"literal prompt", []string{"--", "--remote"}, true}, {"config value", []string{"--config", "--remote"}, true},
		{"options equals", []string{"--model=test", "--no-alt-screen"}, true}, {"missing option value", []string{"--model"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := append([]string{}, tc.args...)
			if tc.isolated {
				want = append([]string{"--no-daemon"}, want...)
			}
			got := Arguments(tc.args)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Arguments(%q)=%q want %q", tc.args, got, want)
			}
			if len(tc.args) > 0 && len(got) > 0 {
				old := tc.args[0]
				got[0] = "mutated"
				if tc.args[0] != old {
					t.Fatal("modified original arguments")
				}
			}
		})
	}
}
