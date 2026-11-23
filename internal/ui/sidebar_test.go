package ui

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"
	"testing"

	"gioui.org/io/input"

	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

func sideRows(first, n int, prefix string) []sideRow {
	var rs []sideRow
	for i := 0; i < n; i++ {
		rs = append(rs, sideRow{ID: tabs.TabID(first + i), Title: fmt.Sprintf("%s%d", prefix, i), Folder: "~/code/app", Branch: "main"})
	}
	return rs
}

// sideDraw lays the sidebar out in a 260 x h box with default theme.
// withSpaces fills the derived Spaces and Agents of a model built by hand.
func withSpaces(m sideModel) sideModel {
	if m.Spaces == nil {
		m.Spaces, m.Agents = buildSpaces(m, 0).Spaces, buildAgents(m).Rows
	}
	return m
}

func sideDraw(t *testing.T, m sideModel, st sideState, h int) (sideHits, image.Point) {
	t.Helper()
	size := image.Pt(sidebarW, h)
	gtx := pageContext(size.X, size.Y)
	return newChrome().layoutSidebar(gtx, withSpaces(m), st, size), size
}

func twoGroupsOneLoose() sideModel {
	return sideModel{
		Groups: []sideGroup{
			{Key: "a", Name: "alpha", Rows: sideRows(1, 2, "a"), Count: 2},
			{Key: "b", Name: "beta", Rows: sideRows(10, 1, "b"), Count: 1, Waiting: 1},
		},
		Loose: sideRows(20, 1, "loose"),
	}
}

func TestSidebarHitsMatchSpaces(t *testing.T) {
	h, size := sideDraw(t, twoGroupsOneLoose(), sideState{}, 600)
	if len(h.rows) != 0 || len(h.spaces) != 3 {
		t.Fatalf("rows %d spaces %d, want 0 (no agents) and 3 (alpha, beta, Shell)", len(h.rows), len(h.spaces))
	}
	box := image.Rectangle{Max: size}
	for _, g := range h.spaces {
		if !g.rect.In(box) || g.rect.Empty() {
			t.Fatalf("space %q rect %v outside %v", g.key, g.rect, box)
		}
	}
	if !h.filter.Empty() {
		t.Fatalf("filter field %v drawn while closed", h.filter)
	}
	for _, r := range []image.Rectangle{h.search, h.plus} {
		if r.Empty() || !r.In(box) {
			t.Fatalf("header icon %v outside %v", r, box)
		}
	}
	open, _ := sideDraw(t, twoGroupsOneLoose(), sideState{filterOpen: true}, 600)
	if open.filter.Empty() || open.spaces[0].rect.Min.Y < open.filter.Max.Y {
		t.Fatalf("open filter %v, first space %v", open.filter, open.spaces[0].rect)
	}
}

func TestSidebarSpacesIgnoreFold(t *testing.T) {
	m := twoGroupsOneLoose()
	m.Groups[0].Folded = true // a space has no fold: its tabs live in the strip
	h, _ := sideDraw(t, m, sideState{}, 600)
	if len(h.spaces) != 3 || h.spaces[0].key != "a" {
		t.Fatalf("spaces %+v, want all three with alpha first", h.spaces)
	}
}

func TestSidebarAgentRowsHaveNoClose(t *testing.T) {
	m := twoGroupsOneLoose()
	for i := range m.Groups[0].Rows {
		m.Groups[0].Rows[i].Agent = "claude"
	}
	h, _ := sideDraw(t, m, sideState{hover: 1}, 600)
	if len(h.rows) != 2 {
		t.Fatalf("%d agent rows, want 2", len(h.rows))
	}
	for _, r := range h.rows {
		if !r.close.Empty() {
			t.Fatalf("agent row %d has a close %v: tabs close in the strip", r.id, r.close)
		}
	}
}

// manyGroups is n one-session spaces named g1, g2, ...
func manyGroups(n int) sideModel {
	var m sideModel
	for i := 1; i <= n; i++ {
		key := "g" + strconv.Itoa(i)
		m.Groups = append(m.Groups, sideGroup{Key: key, Name: key, Rows: sideRows(i, 1, key), Count: 1})
	}
	return m
}

func TestSidebarScrollsAndClampsAt(t *testing.T) {
	m := manyGroups(40)
	h, _ := sideDraw(t, m, sideState{}, 600)
	if h.scrollMax <= 0 {
		t.Fatalf("scrollMax %d with 40 spaces in 600px", h.scrollMax)
	}
	if first := h.spaces[0]; first.key != "g1" {
		t.Fatalf("first visible space %q at scroll 0", first.key)
	}
	for _, r := range h.spaces {
		if !r.rect.In(h.list) {
			t.Fatalf("space %q rect %v leaks out of list %v", r.key, r.rect, h.list)
		}
	}
	far, _ := sideDraw(t, m, sideState{scroll: 1 << 20}, 600)
	last := far.spaces[len(far.spaces)-1]
	if last.key != "g40" {
		t.Fatalf("clamped scroll ends at %q, want the last space g40", last.key)
	}
	if want := far.list.Max.Y - gtx0Dp(sideSpaceGap); last.rect.Max.Y != want {
		t.Fatalf("last space ends at %d, want %d: not clamped to the content", last.rect.Max.Y, want)
	}
	for _, r := range far.spaces {
		if r.key == "g1" {
			t.Fatal("a space scrolled above the list is still hit")
		}
		if !r.rect.In(far.list) {
			t.Fatalf("space %q rect %v leaks out of list %v", r.key, r.rect, far.list)
		}
	}
}

func fourMemberRun() sideRun {
	r := sideRun{Run: 7, Name: "dev stack", Down: 1}
	for i, st := range []string{"Running", "Running", "stopped", "Starting"} {
		r.Members = append(r.Members, sideMember{ID: tabs.TabID(100 + i), Title: fmt.Sprintf("srv%d", i), Color: "green", Status: st, Down: st == "stopped"})
	}
	return r
}

func TestSidebarSectionAtTheBottom(t *testing.T) {
	m := twoGroupsOneLoose()
	m.Runs = []sideRun{fourMemberRun()}
	h, size := sideDraw(t, m, sideState{}, 600)
	if len(h.members) != 4 || len(h.runs) != 1 {
		t.Fatalf("members %d runs %d, want 4 and 1", len(h.members), len(h.runs))
	}
	lastRow := 0
	for _, r := range h.rows {
		lastRow = max(lastRow, r.rect.Max.Y)
	}
	for _, mem := range h.members {
		if mem.rect.Min.Y < lastRow || mem.rect.Max.Y > size.Y {
			t.Fatalf("member %d at %v, rows end at %d, sidebar %d high", mem.id, mem.rect, lastRow, size.Y)
		}
	}
	if h.list.Max.Y > h.members[0].rect.Min.Y {
		t.Fatalf("list %v overlaps the section at %v", h.list, h.members[0].rect)
	}
}

func TestSidebarSettingsRowFirst(t *testing.T) {
	h, _ := sideDraw(t, twoGroupsOneLoose(), sideState{settings: true, settingsOn: true}, 600)
	if h.settings.Empty() || h.settingsClose.Empty() {
		t.Fatalf("settings %v close %v", h.settings, h.settingsClose)
	}
	if h.settings.Max.Y > h.spaces[0].rect.Min.Y {
		t.Fatalf("settings %v not above the first space %v", h.settings, h.spaces[0].rect)
	}
	none, _ := sideDraw(t, twoGroupsOneLoose(), sideState{}, 600)
	if !none.settings.Empty() {
		t.Fatalf("settings rect %v while closed", none.settings)
	}
}

func TestSidebarLongTitleStaysInside(t *testing.T) {
	m := sideModel{
		Groups: []sideGroup{{Key: "g", Name: strings.Repeat("space", 60), Count: 1, Rows: []sideRow{
			{ID: 3, Agent: "claude", Title: strings.Repeat("w", 200), Command: strings.Repeat("c", 200)}}}},
		Loose: []sideRow{
			{ID: 1, Agent: "codex", Title: strings.Repeat("w", 200), Folder: strings.Repeat("~/dir", 40), Branch: strings.Repeat("b", 200)},
			{ID: 2, Agent: "claude", Title: strings.Repeat("日本語のタイトル", 30), Folder: "~/日本", Branch: "main", Mark: markNeeds},
		},
	}
	m.Runs = []sideRun{{Run: 1, Name: strings.Repeat("n", 120), Members: []sideMember{{ID: 5, Title: strings.Repeat("長", 100), Status: "Running"}}}}
	h, size := sideDraw(t, m, sideState{hover: 1, filterOpen: true, filter: strings.Repeat("f", 200)}, 400)
	box := image.Rectangle{Max: size}
	if len(h.rows) != 3 || len(h.spaces) != 2 {
		t.Fatalf("%d agent rows and %d spaces, want 3 and 2", len(h.rows), len(h.spaces))
	}
	for _, r := range h.rows {
		if !r.rect.In(box) {
			t.Fatalf("agent row %d rect %v outside %v", r.id, r.rect, box)
		}
	}
	for _, r := range h.spaces {
		if !r.rect.In(box) {
			t.Fatalf("space %q rect %v outside %v", r.key, r.rect, box)
		}
	}
}

func TestSidebarDrawsInEveryTheme(t *testing.T) {
	for _, th := range theme.All() {
		t.Run(th.Key, func(t *testing.T) {
			c := newChrome()
			c.setTheme(th)
			m := twoGroupsOneLoose()
			m.Runs = []sideRun{fourMemberRun()}
			size := image.Pt(sidebarW, 600)
			h := c.layoutSidebar(pageContext(size.X, size.Y), withSpaces(m), sideState{space: "a", settings: true}, size)
			if len(h.spaces) == 0 {
				t.Fatal("nothing drawn")
			}
			if th.UI.Active == th.UI.Chrome {
				t.Fatalf("selected row fill %v equals the sidebar background", th.UI.Active)
			}
		})
	}
}

func TestSidebarFoldedRunHasNoMembers(t *testing.T) {
	m := sideModel{Runs: []sideRun{fourMemberRun()}}
	open, _ := sideDraw(t, m, sideState{}, 600)
	folded, _ := sideDraw(t, m, sideState{runFolded: map[uint64]bool{7: true}}, 600)
	if len(open.members) != 4 || len(folded.members) != 0 {
		t.Fatalf("members open %d folded %d, want 4 and 0", len(open.members), len(folded.members))
	}
	if len(folded.runs) != 1 || folded.runs[0].run != 7 {
		t.Fatalf("folded run header hit %+v", folded.runs)
	}
	if folded.list.Max.Y <= open.list.Max.Y {
		t.Fatalf("folded section did not shrink: list ends %d vs %d", folded.list.Max.Y, open.list.Max.Y)
	}
}

func TestSidebarMemberWithoutShellHasNoHit(t *testing.T) {
	r := fourMemberRun()
	r.Members[1].ID = 0
	h, _ := sideDraw(t, sideModel{Runs: []sideRun{r}}, sideState{}, 600)
	if len(h.members) != 3 {
		t.Fatalf("%d member hits, want 3", len(h.members))
	}
}

func TestSidebarCappedSectionKeepsRoomForMore(t *testing.T) {
	r := sideRun{Run: 1, Name: "big"}
	for i := 0; i < 30; i++ {
		r.Members = append(r.Members, sideMember{ID: tabs.TabID(100 + i), Title: "m", Status: "Running"})
	}
	h, size := sideDraw(t, sideModel{Runs: []sideRun{r}}, sideState{}, 400)
	if len(h.members) == 0 || len(h.members) >= 30 {
		t.Fatalf("%d member hits in a capped section", len(h.members))
	}
	for _, m := range h.members {
		if m.rect.Max.Y > size.Y-24 {
			t.Fatalf("member %v leaves no room for the +N more line", m.rect)
		}
	}
}

// The running ring is drawn still: laying a running row out never asks for another frame.
func TestRunningMarkIsStill(t *testing.T) {
	m := sideModel{Loose: []sideRow{{ID: 1, Title: "V3-1", Command: "/ship-ticket", Running: true}}}
	c := newChrome()
	r := new(input.Router)
	for i := 0; i < 2; i++ {
		gtx := pageContext(sidebarW, 400)
		gtx.Source = r.Source()
		c.layoutSidebar(gtx, m, sideState{}, image.Pt(sidebarW, 400))
		r.Frame(gtx.Ops)
		if _, wake := r.WakeupTime(); wake {
			t.Fatalf("frame %d asked for another frame", i)
		}
	}
}

func TestRunHeaderStateSaysReadyNotAllUp(t *testing.T) {
	th := theme.Default().UI
	for _, c := range []struct {
		down, ready int
		want        string
		col         color.NRGBA
	}{
		{0, 0, "all up", th.Done},
		{0, 1, "1 ready", th.Muted},
		{0, 2, "2 ready", th.Muted},
		{1, 2, "1 down", th.UsageHigh}, // down wins
	} {
		s, col := runHeaderState(sideRun{Down: c.down, Ready: c.ready}, th)
		if s != c.want || col != c.col {
			t.Fatalf("down %d ready %d reads %q %v, want %q %v", c.down, c.ready, s, col, c.want, c.col)
		}
	}
}
