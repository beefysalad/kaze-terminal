package main

import (
	"github.com/beefysalad/kaze-terminal/internal/control"
	"testing"
	"time"
)

func TestControlConsentReadBudget(t *testing.T) {
	for _, tc := range []struct {
		req  control.Request
		want time.Duration
	}{
		{control.Request{Op: "spawn"}, 70 * time.Second},
		{control.Request{Op: "spawn", WaitStart: true}, 105 * time.Second},
		{control.Request{Op: "read"}, controlTimeout},
		{control.Request{Op: "kill"}, controlTimeout},
	} {
		if got := controlReadBudget(tc.req); got != tc.want {
			t.Fatalf("%+v budget=%v want %v", tc.req, got, tc.want)
		}
	}
}
