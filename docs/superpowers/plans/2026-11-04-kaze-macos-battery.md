# macOS battery (#139): plan

Spec: `docs/superpowers/specs/2026-11-04-kaze-macos-battery-design.md`. Branch `feat/macos-battery`.
Strict TDD: each step's test fails first (RED), then passes (GREEN).

1. `fromIOPS` in `battery.go`: table test first (no battery, zero max, 71 on battery, 64 charging,
   80 on AC without the charging flag, 100 on AC no bolt, rounding, clamp), then the function.
2. Narrow `source_other.go` and `source_other_test.go` to `!windows && !darwin`.
3. Darwin source: tests first (real read sanity, benchmark, faked watch cases, real watch smoke), then
   `source_darwin.go` + `iops_darwin.c`.
4. Gates: `go vet ./...`, `go test ./...`; benchmark 0 allocs/op.
5. Hand check on the MacBook: build, run, battery at the far right; unplug and replug the charger and
   see the bolt follow.
6. Docs: README battery section and 0.28 roadmap line, HANDOFF status / known gaps / Mac checklist.
7. PR. After the owner merges: release 0.29 per CLAUDE.md.
