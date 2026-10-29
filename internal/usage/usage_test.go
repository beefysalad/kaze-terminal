package usage

import (
	"testing"
	"time"

	"github.com/patrick-gogo/kaze-terminal/internal/notify"
)

var now = time.Date(2026, 10, 29, 9, 0, 0, 0, time.Local)

func pct(v float64) *float64 { return &v }

func win(p float64, resets time.Time) *notify.Window {
	w := &notify.Window{Pct: p}
	if !resets.IsZero() {
		w.Resets = resets.Unix()
	}
	return w
}

func texts(s []Segment) []string {
	out := make([]string, len(s))
	for i, x := range s {
		out[i] = x.Text
	}
	return out
}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestSegmentsOrderAndRounding(t *testing.T) {
	u := notify.Usage{Ctx: pct(34.9), Week: win(41.2, time.Time{}), FiveHour: win(23.5, now.Add(time.Hour))}
	eq(t, texts(Segments(u, now)), []string{"5h 23%", "wk 41%", "ctx 34%"})
	for _, s := range Segments(u, now) {
		if s.Level != Normal {
			t.Fatalf("level %v for %q", s.Level, s.Text)
		}
	}
}

func TestSegmentsClamp(t *testing.T) {
	u := notify.Usage{FiveHour: win(-5, time.Time{}), Week: win(250, time.Time{})}
	got := Segments(u, now)
	eq(t, texts(got), []string{"5h 0%", "wk 100%"})
	if got[1].Level != High {
		t.Fatalf("clamped 100 should be High, got %v", got[1].Level)
	}
}

func TestSegmentsAbsentFields(t *testing.T) {
	eq(t, texts(Segments(notify.Usage{}, now)), []string{})
	eq(t, texts(Segments(notify.Usage{Ctx: pct(8)}, now)), []string{"ctx 8%"})
}

func TestExpiredWindowHidden(t *testing.T) {
	u := notify.Usage{FiveHour: win(50, now), Week: win(10, now.Add(-time.Minute))}
	eq(t, texts(Segments(u, now)), []string{})
}

func TestLevelsAndResetText(t *testing.T) {
	soon := time.Date(2026, 10, 29, 15, 40, 0, 0, time.Local)
	later := time.Date(2026, 11, 2, 8, 5, 0, 0, time.Local) // a Monday
	u := notify.Usage{FiveHour: win(80, soon), Week: win(95.2, later), Ctx: pct(97)}
	got := Segments(u, now)
	eq(t, texts(got), []string{"5h 80% · resets 3:40 PM", "wk 95% · resets Mon 8:05 AM", "ctx 97%"})
	if got[0].Level != Warn || got[1].Level != High || got[2].Level != High {
		t.Fatalf("levels %v %v %v", got[0].Level, got[1].Level, got[2].Level)
	}
	if s := Segments(notify.Usage{FiveHour: win(79.9, soon)}, now); s[0].Level != Normal {
		t.Fatalf("79.9 should be Normal")
	}
}

func TestLine(t *testing.T) {
	u := notify.Usage{FiveHour: win(23, time.Time{}), Ctx: pct(34)}
	if got := Line(u, now); got != "5h 23%  ·  ctx 34%" {
		t.Fatalf("got %q", got)
	}
	if got := Line(notify.Usage{}, now); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}

func TestNextReset(t *testing.T) {
	a, b := now.Add(2*time.Hour), now.Add(time.Hour)
	u := notify.Usage{FiveHour: win(1, a), Week: win(1, b)}
	if got := NextReset(u, now); !got.Equal(time.Unix(b.Unix(), 0)) {
		t.Fatalf("got %v", got)
	}
	past := notify.Usage{FiveHour: win(1, now.Add(-time.Hour)), Week: win(1, time.Time{})}
	if got := NextReset(past, now); !got.IsZero() {
		t.Fatalf("want zero, got %v", got)
	}
}
