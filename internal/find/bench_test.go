package find

import (
	"fmt"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/screen"
)

const benchCols, benchRows = 200, 50

// logScreen is a 200 x 50 screen whose history holds n log-like lines of 120 to 200
// columns, every 37th with a WARN. Built with a fixed loop.
func logScreen(n int) *screen.Screen {
	scr := screen.NewWithScrollback(benchCols, benchRows, n)
	var b strings.Builder
	for i := 0; i < n+benchRows; i++ {
		b.Reset()
		level := "INFO "
		if i%37 == 0 {
			level = "WARN "
		}
		fmt.Fprintf(&b, "2026-11-04T12:%02d:%02d.%03dZ %s api  GET /v1/items/%d 200 %dms user=%d trace=%016x",
			i/3600%60, i/60%60, i%1000, level, i, i%250, i%9973, uint64(i)*2654435761)
		for b.Len() < 120+i%81 {
			b.WriteString(" k=v")
		}
		if i > 0 {
			scr.CarriageReturn()
			scr.LineFeed()
		}
		for _, r := range b.String()[:min(b.Len(), benchCols)] {
			scr.Print(r)
		}
	}
	return scr
}

var bench10k, bench100k *screen.Screen

func screen10k() *screen.Screen {
	if bench10k == nil {
		bench10k = logScreen(10_000)
	}
	return bench10k
}

func screen100k() *screen.Screen {
	if bench100k == nil {
		bench100k = logScreen(100_000)
	}
	return bench100k
}

// fullScan restarts the search (the needle flips between two values) and steps it
// with StepBudget until it settles, reporting cells read and steps taken per op.
func fullScan(b *testing.B, scr *screen.Screen) {
	var s Search
	s.SetQuery("warn", false)
	for n := 0; n < 1000 && !s.Step(scr, StepBudget); n++ {
	}
	queries := [2]string{"warn", "wArN"}
	b.ReportAllocs()
	b.ResetTimer()
	cells, steps := s.Scanned(), 0
	for i := 0; i < b.N; i++ {
		s.SetQuery(queries[i&1], i&2 != 0) // always a change: the text or the case flips
		steps++
		for n := 0; n < 1000 && !s.Step(scr, StepBudget); n++ {
			steps++
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(s.Scanned()-cells)/float64(b.N), "cells/op")
	b.ReportMetric(float64(steps)/float64(b.N), "steps/op")
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(s.Scanned()-cells), "ns/cell")
}

func BenchmarkFullScan10k(b *testing.B)  { fullScan(b, screen10k()) }
func BenchmarkFullScan100k(b *testing.B) { fullScan(b, screen100k()) }

// BenchmarkStepBudget times one Step of StepBudget cells, the most the session lock
// is held for find in one frame (Ruling 7: under 5 ms).
func BenchmarkStepBudget(b *testing.B) {
	scr := screen100k()
	var s Search
	queries := [2]string{"warn", "wArN"}
	s.SetQuery(queries[0], false)
	s.Step(scr, StepBudget)
	b.ReportAllocs()
	b.ResetTimer()
	cells := s.Scanned()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		s.SetQuery(queries[i&1], i&2 != 0)
		b.StartTimer()
		s.Step(scr, StepBudget)
	}
	b.StopTimer()
	b.ReportMetric(float64(s.Scanned()-cells)/float64(b.N), "cells/op")
}

// BenchmarkOutputRefresh is one new line on a settled 10,000-line search.
func BenchmarkOutputRefresh(b *testing.B) {
	scr := logScreen(10_000) // its own: the benchmark prints into it
	var s Search
	s.SetQuery("warn", false)
	for n := 0; n < 1000 && !s.Step(scr, StepBudget); n++ {
	}
	line := "2026-11-04T12:00:00.000Z WARN  api  GET /v1/items/1 500 12ms user=1 trace=0000000000000001"
	b.ReportAllocs()
	b.ResetTimer()
	cells := s.Scanned()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		scr.CarriageReturn()
		scr.LineFeed()
		for _, r := range line {
			scr.Print(r)
		}
		b.StartTimer()
		s.Output(scr)
		for n := 0; n < 1000 && !s.Step(scr, StepBudget); n++ {
		}
		s.Matches()
		s.Spans()
	}
	b.StopTimer()
	b.ReportMetric(float64(s.Scanned()-cells)/float64(b.N), "cells/op")
}
