package main

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
)

// A -H=windowsgui build has no console, so a fatal error must reach the user through showError.
func TestFatalErrorIsShownAndLogged(t *testing.T) {
	var title, text string
	calls := 0
	old := showError
	showError = func(ti, te string) { calls++; title, text = ti, te }
	t.Cleanup(func() { showError = old })
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	reportFatal(errors.New(`start "[pwsh]": exec: not found`))

	if calls != 1 || title != "kaze" || !strings.Contains(text, `start "[pwsh]": exec: not found`) {
		t.Fatalf("showError called %d times with %q / %q", calls, title, text)
	}
	if !strings.Contains(buf.String(), "exec: not found") {
		t.Fatalf("stderr log = %q, want the error", buf.String())
	}
}
