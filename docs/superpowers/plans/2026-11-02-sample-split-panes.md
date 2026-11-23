# Split panes: implementation plan (SAMPLE, delete me)

> **For Claude:** use superpowers:executing-plans to run this plan task by task.

**Goal:** Two panes per tab. **Design:** [split panes design](../specs/2026-11-02-sample-split-panes-design.md)

**Tech stack:** Go 1.27, Gio, `internal/term` sessions.

---

## Task 1: Pane type

**Files:**
- Create: `internal/tabs/pane.go`
- Test: `internal/tabs/pane_test.go`

**Step 1: Write the failing test**

```go
func TestNewPaneOwnsSession(t *testing.T) {
	p := NewPane(fakeSession())
	if p.Session() == nil {
		t.Fatal("pane has no session")
	}
}
```

**Step 2: Run it, expect FAIL**

Run: `go test ./internal/tabs -run TestNewPaneOwnsSession`
Expected: `undefined: NewPane`

**Step 3: Implement**

```go
type Pane struct{ s *term.Session }

func NewPane(s *term.Session) *Pane    { return &Pane{s: s} }
func (p *Pane) Session() *term.Session { return p.s }
```

**Step 4: Commit**

```bash
git commit -m "feat(tabs): Add Pane type"
```

## Task 2: Split ratio

- [ ] Write `TestRatioClamps` (0.1 → 0.2, 0.9 → 0.8)
- [ ] Add `Tab.SetRatio`
- [ ] Commit

## Task 3: Layout in the UI

- [ ] Split the content rect in `internal/ui`
- [ ] Draw the divider and focus border
- [ ] Route keys to the focused pane

## Task 4: Keys and close rules

| Test | Expect |
|---|---|
| `Cmd+D` on one pane | two panes, focus on new |
| `Cmd+W` on last pane | tab closes |

## Done when

1. `go test ./...` and `go vet ./...` pass.
2. Bench stays under 16 ms per frame.
3. Owner checks the [design's testing list](../specs/2026-11-02-sample-split-panes-design.md#7-testing).
