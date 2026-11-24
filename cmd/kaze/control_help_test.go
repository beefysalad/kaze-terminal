package main

import (
	"bytes"
	"strings"
	"testing"
)

// An agent that guesses a wrong subcommand inside a tab must get usage text,
// never a second Kaze window.
func TestControlHelpAndUnknownCommandsNeverStartTheApp(t *testing.T) {
	run := func(args ...string) (bool, int, string, string) {
		var out, errb bytes.Buffer
		handled, code := dispatchControl(args, func(string) string { return "" }, strings.NewReader(""), &out, &errb)
		return handled, code, out.String(), errb.String()
	}
	for _, arg := range []string{"--help", "-h", "help"} {
		handled, code, out, _ := run(arg)
		if !handled || code != 0 || !strings.Contains(out, "usage: kaze ls") {
			t.Fatalf("%s: handled=%v code=%d out=%q", arg, handled, code, out)
		}
	}
	for _, arg := range []string{"list", "status", "new-tab", "Codex"} {
		handled, code, _, errb := run(arg)
		if !handled || code != 2 || !strings.Contains(errb, "unknown command") || !strings.Contains(errb, "usage: kaze ls") {
			t.Fatalf("%s: handled=%v code=%d stderr=%q", arg, handled, code, errb)
		}
	}
	// The app's own arguments still reach the window code.
	for _, args := range [][]string{nil, {"--workspace", "x"}, {"--workspace=x"}, {"-psn_0_12345"}} {
		if handled, _, _, _ := run(args...); handled {
			t.Fatalf("%v must not be handled by the control CLI", args)
		}
	}
}
