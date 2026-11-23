# Split panes: design (SAMPLE, delete me)

> Sample spec made to test the markdown reader. Not a real plan.

**Status:** approved by owner 2026-11-02
**Plan:** [2026-11-02-sample-split-panes.md](../plans/2026-11-02-sample-split-panes.md)

## 1. Goal

Let one tab show two terminals side by side, so Claude can run in one pane while tests run in the other.

## 2. Non-goals

- More than two panes per tab.
- Drag-and-drop panes between tabs.

## 3. User experience

| Action | Key (macOS) | Key (Windows) |
|---|---|---|
| Split right | `Cmd+D` | `Ctrl+Shift+D` |
| Split down | `Cmd+Shift+D` | `Ctrl+Shift+E` |
| Move focus | `Cmd+Opt+Arrow` | `Alt+Arrow` |
| Close pane | `Cmd+W` | `Ctrl+Shift+W` |

- The focused pane gets a 1 px accent border.
- Dragging the divider resizes; the ratio is clamped to 20%..80%.

## 4. Architecture

```
Tab
 ├── Pane A ── Session (PTY + parser + screen)
 └── Pane B ── Session
```

- `internal/tabs`: new `Pane` type; `Tab` holds `[]*Pane` and `Ratio float32`.
- `internal/ui`: layout splits the content rect; input routes to the focused pane.
- `internal/render`: unchanged, it already draws one grid into a rect.

## 5. Decisions

1. **Ratio is per tab**, not global. Different tabs have different jobs.
2. **Closing the last pane closes the tab**, matching today's behaviour.
3. See the [risks](#6-risks) before starting.

## 6. Risks

- Resize storms while dragging (debounce PTY resize to 50 ms).
- Focus bugs when a pane's process exits.

## 7. Testing

- [ ] Unit: ratio clamping, focus moves, close rules.
- [ ] Bench: two panes of `cat big.txt` stay under 16 ms per frame.
- [ ] Manual Mac checklist in [HANDOFF](../../HANDOFF.md).
