package ui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/skills"
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
)

func palNames(items []palItem) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Name)
	}
	return out
}

// palGroups are the headers drawn: each Group where it changes.
func palGroups(items []palItem) []string {
	var out []string
	for i, it := range items {
		if i == 0 || items[i-1].Group != it.Group {
			out = append(out, it.Group)
		}
	}
	return out
}

func palOf(items []palItem, group string, kind palKind) []palItem {
	var out []palItem
	for _, it := range items {
		if it.Group == group && it.Kind == kind {
			out = append(out, it)
		}
	}
	return out
}

func palFind(items []palItem, key string) (palItem, bool) {
	for _, it := range items {
		if it.Key == key {
			return it, true
		}
	}
	return palItem{}, false
}

// samplePal: one session needing you, three others, ten skills (3 yours, 6
// devbox, 1 superpowers), two recent and four actions.
func samplePal() palInput {
	list := []skills.Skill{
		{Name: "review-mine", Desc: "Review your own branch", Scope: skills.User},
		{Name: "ship-ticket", Desc: "Open the PR for a ticket; asks for review", Scope: skills.User},
		{Name: "ci-watch", Desc: "Babysit CI on my PRs", Scope: skills.User},
		{Name: "devbox:run", Desc: "Send a task to a worker", Scope: skills.Plugin},
		{Name: "devbox:dash", Desc: "Open the board", Scope: skills.Plugin},
		{Name: "devbox:kit", Desc: "What a job lacks", Scope: skills.Plugin},
		{Name: "devbox:lead", Desc: "Send a bigger task", Scope: skills.Plugin},
		{Name: "devbox:jobs", Desc: "Your jobs", Scope: skills.Plugin},
		{Name: "devbox:z6", Desc: "The sixth", Scope: skills.Plugin},
		{Name: "superpowers:writing-plans", Desc: "Plan it in steps", Scope: skills.Plugin},
	}
	return palInput{
		Sessions: []sideRow{
			{ID: 1, Title: "V3-3348", Folder: "~/code/beta", Branch: "v3-3348", Command: "/ship-ticket", Mark: markNeeds, Claude: true},
			{ID: 2, Title: "api", Folder: "~/code/api", Running: true, Claude: true},
			{ID: 3, Title: "misc", Folder: "~/notes"},
			{ID: 4, Title: "codex", Folder: "~/code/web", Agent: "codex"},
		},
		Skills: list,
		Groups: groupSkills(list),
		Recent: []string{"/ci-watch", "/review-mine"},
		Actions: []palAction{
			{Name: "New session", Config: "new_tab", Act: actNewTab, Keys: []string{"Ctrl", "Shift", "T"}, Icon: "plus"},
			{Name: "Open settings", Config: "open_settings", Act: actOpenSettings, Keys: []string{"Ctrl", ","}, Icon: "cog"},
			{Name: "Find in the terminal", Config: "find", Act: actFind, Keys: []string{"Ctrl", "Shift", "F"}, Icon: "search"},
			{Name: "Toggle sidebar", Config: "toggle_sidebar", Act: actToggleSidebar, Keys: []string{"Ctrl", "B"}, Icon: "panel"},
		},
	}
}

func withQuery(q string) palInput {
	in := samplePal()
	in.Query = q
	return in
}

func TestEmptySessionsScope(t *testing.T) {
	got := buildPalette(samplePal())
	if fmt.Sprint(palGroups(got)) != "[WAITING ON YOU SESSIONS RECENT SKILLS]" {
		t.Fatalf("groups %v", palGroups(got))
	}
	rec := palOf(got, palHeadRecentSkills, palSkill)
	if fmt.Sprint(palNames(rec)) != "[/ci-watch /review-mine]" || rec[0].Key != "recent:ci-watch" || rec[0].Prompt != "/ci-watch" {
		t.Fatalf("recent %+v", rec)
	}
	last := got[len(got)-1]
	if last.Kind != palBrowse || last.Name != "Browse all 10 skills" || last.Count != 10 || last.Key != "browse" || last.Group != palHeadRecentSkills {
		t.Fatalf("last row %+v", last)
	}
	for _, it := range got {
		if it.Kind == palKaze {
			t.Fatalf("an action on the empty field: %+v", it)
		}
	}
	if len(palOf(got, palHeadSessions, palSession)) != 3 {
		t.Fatalf("sessions %v", palNames(got))
	}
}

func TestNoRecentNoBrowseWithoutSkills(t *testing.T) {
	in := samplePal()
	in.Recent = nil
	got := buildPalette(in)
	last := got[len(got)-1]
	if last.Kind != palBrowse || last.Group != palHeadSkills {
		t.Fatalf("browse without recent %+v", last)
	}
	in.Skills, in.Groups = nil, nil
	for _, it := range buildPalette(in) {
		if it.Kind == palBrowse || it.Kind == palSkill {
			t.Fatalf("no skills, yet %+v", it)
		}
	}
}

func TestSessionsScopeText(t *testing.T) {
	in := withQuery("r")
	got := buildPalette(in)
	groups := palGroups(got)
	ki, si := -1, -1
	for i, g := range groups {
		switch g {
		case palHeadKaze:
			ki = i
		case palHeadSkills:
			si = i
		}
	}
	if ki < 0 || si < 0 || ki > si {
		t.Fatalf("groups %v: want KAZE before SKILLS", groups)
	}
	total := 0
	for _, s := range in.Skills {
		if _, ok := palTier("r", s.Name, s.Desc); ok {
			total++
		}
	}
	if total <= 3 {
		t.Fatalf("fixture: %d matches", total)
	}
	if n := len(palOf(got, palHeadSkills, palSkill)); n != 3 {
		t.Fatalf("%d skill rows, want 3", n)
	}
	more, ok := palFind(got, "more:matches")
	if !ok || more.Kind != palMoreMatches || more.Count != total-3 || more.Query != "/r" || more.Name != strconv.Itoa(total-3)+" more matching skills" {
		t.Fatalf("more row %+v (ok %v), total %d", more, ok, total)
	}
	if got[len(got)-1].Key != "more:matches" {
		t.Fatalf("more row is not last: %v", palNames(got))
	}
	for _, it := range palOf(got, palHeadSkills, palSkill) {
		if it.GroupCount != strconv.Itoa(total) {
			t.Fatalf("SKILLS count %q, want %d", it.GroupCount, total)
		}
	}
}

func TestActionMatchesShortcut(t *testing.T) {
	got := buildPalette(withQuery("shift+t"))
	it, ok := palFind(got, "act:new_tab")
	if !ok || it.Kind != palKaze || it.Group != palHeadKaze || it.Icon != "plus" || strings.Join(it.Keys, "+") != "Ctrl+Shift+T" {
		t.Fatalf("new session %+v (ok %v) in %v", it, ok, palNames(got))
	}
}

func TestEmptySkillsScope(t *testing.T) {
	got := buildPalette(withQuery("/"))
	if fmt.Sprint(palGroups(got)) != "[RECENT YOURS DEVBOX SUPERPOWERS]" {
		t.Fatalf("groups %v", palGroups(got))
	}
	if fmt.Sprint(palNames(palOf(got, palHeadRecent, palSkill))) != "[/ci-watch /review-mine]" {
		t.Fatalf("recent %v", palNames(got))
	}
	if fmt.Sprint(palNames(palOf(got, "YOURS", palSkill))) != "[/ci-watch /review-mine /ship-ticket]" {
		t.Fatalf("yours %v", palNames(got))
	}
	dev := palOf(got, "DEVBOX", palSkill)
	if fmt.Sprint(palNames(dev)) != "[/devbox:dash /devbox:jobs /devbox:kit /devbox:lead]" {
		t.Fatalf("devbox %v", palNames(dev))
	}
	if dev[0].GroupCount != "6" || dev[0].Plugin != "devbox" {
		t.Fatalf("devbox row %+v", dev[0])
	}
	more, ok := palFind(got, "more:p:devbox")
	if !ok || more.Kind != palMoreGroup || more.Name != "2 more in devbox" || more.Count != 2 || more.Plugin != "devbox" || more.Group != "DEVBOX" {
		t.Fatalf("more row %+v (ok %v)", more, ok)
	}
	for _, it := range got {
		if it.Kind == palSession || it.Kind == palKaze || it.Kind == palBrowse {
			t.Fatalf("Skills scope lists %+v", it)
		}
	}
}

func TestOpenGroupShowsAll(t *testing.T) {
	in := withQuery("/")
	in.Open = map[string]bool{"p:devbox": true}
	got := buildPalette(in)
	if n := len(palOf(got, "DEVBOX", palSkill)); n != 6 {
		t.Fatalf("%d devbox rows, want 6", n)
	}
	if _, ok := palFind(got, "more:p:devbox"); ok {
		t.Fatal("an open group still has its more row")
	}
}

func TestOtherFoldsLikeAPlugin(t *testing.T) {
	var list []skills.Skill
	for i := range 6 {
		list = append(list, skills.Skill{Name: fmt.Sprintf("orphan-%d", i), Scope: skills.Plugin})
	}
	got := buildPalette(palInput{Query: "/", Skills: list, Groups: groupSkills(list)})
	more, ok := palFind(got, "more:Other")
	if len(palOf(got, "OTHER", palSkill)) != palFoldAt || !ok || more.Name != "2 more in Other" || more.Fold != "Other" {
		t.Fatalf("other %v, more %+v", palNames(got), more)
	}
}

func TestSkillsScopeTextUnfolds(t *testing.T) {
	got := buildPalette(withQuery("/d"))
	if fmt.Sprint(palGroups(got)) != "[DEVBOX]" {
		t.Fatalf("groups %v", palGroups(got))
	}
	dev := palOf(got, "DEVBOX", palSkill)
	if len(dev) != 6 || dev[0].GroupCount != "6" {
		t.Fatalf("devbox %v", palNames(dev))
	}
	in := withQuery("/d")
	in.Skills = append(in.Skills, skills.Skill{Name: "dig", Desc: "", Scope: skills.User}, skills.Skill{Name: "aaa", Desc: "does it", Scope: skills.User})
	in.Groups = groupSkills(in.Skills)
	yours := palOf(buildPalette(in), "YOURS", palSkill)
	if fmt.Sprint(palNames(yours)) != "[/dig /aaa]" || yours[0].GroupCount != "2" {
		t.Fatalf("yours %v", palNames(yours))
	}
}

func TestRecentKeysAreUnique(t *testing.T) {
	got := buildPalette(withQuery("/"))
	seen := map[string]bool{}
	for _, it := range got {
		if seen[it.Key] {
			t.Fatalf("duplicate key %s", it.Key)
		}
		seen[it.Key] = true
	}
	if !seen["recent:ci-watch"] || !seen["skill:ci-watch"] {
		t.Fatalf("keys %v", seen)
	}
}

func TestPluginSkillDim(t *testing.T) {
	run, _ := palFind(buildPalette(withQuery("/run")), "skill:devbox:run")
	ci, _ := palFind(buildPalette(withQuery("/")), "skill:ci-watch")
	if run.Dim != len("/devbox:") || run.Name != "/devbox:run" || ci.Dim != 0 || ci.Name != "/ci-watch" {
		t.Fatalf("dims %d %d", run.Dim, ci.Dim)
	}
}

func TestSkillScopeIgnoresLeadingSpace(t *testing.T) {
	if !palSkillScope(" /x") || !palSkillScope("/") || palSkillScope("x/") || palSkillScope("") {
		t.Fatal("palSkillScope")
	}
	got := buildPalette(withQuery("  /"))
	if len(got) == 0 || got[0].Group != palHeadRecent {
		t.Fatalf("groups %v", palGroups(got))
	}
}

func TestNeedsSessionOnlyUnderWaiting(t *testing.T) {
	n := 0
	for _, it := range buildPalette(samplePal()) {
		if it.Tab == 1 {
			n++
			if it.Group != palHeadWaiting {
				t.Fatalf("needs session under %q", it.Group)
			}
		}
	}
	if n != 1 {
		t.Fatalf("needs session shown %d times", n)
	}
}

func TestSessionState(t *testing.T) {
	in := samplePal()
	in.Sessions = append(in.Sessions, sideRow{ID: 5, Title: "idle", Claude: true}, sideRow{ID: 6, Title: "fin", Mark: markDone, Claude: true})
	want := map[string][2]string{
		"V3-3348": {"needs you", "claude"}, "api": {"running", "claude"}, "misc": {"shell", ""},
		"codex": {"idle", "codex"}, "idle": {"idle", "claude"}, "fin": {"done", "claude"},
	}
	seen := 0
	for _, it := range buildPalette(in) {
		if it.Kind != palSession {
			continue
		}
		seen++
		if w := want[it.Name]; it.State != w[0] || it.Agent != w[1] {
			t.Errorf("%s: state %q agent %q, want %v", it.Name, it.State, it.Agent, w)
		}
	}
	if seen != len(want) {
		t.Fatalf("%d sessions", seen)
	}
}

func TestPaletteQueryContainsAnywhere(t *testing.T) {
	in := withQuery("ticket")
	got := palNames(buildPalette(in))
	// V3-3348 by its command, /ship-ticket by name.
	if strings.Join(got, ",") != "V3-3348,/ship-ticket" {
		t.Fatalf("ticket: %v", got)
	}
	in.Query = "SHIP"
	if got := palNames(buildPalette(in)); len(got) != 2 {
		t.Fatalf("SHIP: %v", got)
	}
}

func TestPaletteRanksNamePrefixFirst(t *testing.T) {
	in := palInput{Query: "rev", Skills: []skills.Skill{
		{Name: "a-skill", Desc: "nothing here"},
		{Name: "ship-ticket", Desc: "asks for review"},
		{Name: "prevent", Desc: ""},
		{Name: "review-mine", Desc: ""},
	}}
	got := palNames(buildPalette(in))
	if strings.Join(got, ",") != "/review-mine,/prevent,/ship-ticket" {
		t.Fatalf("got %v", got)
	}
}

func TestPluginNameRanksByItsOwnPart(t *testing.T) {
	in := palInput{Query: "brain", Skills: []skills.Skill{
		{Name: "a-brainy", Desc: ""},
		{Name: "superpowers:brainstorming", Desc: "", Scope: skills.Plugin},
	}}
	got := palNames(buildPalette(in))
	if strings.Join(got, ",") != "/superpowers:brainstorming,/a-brainy" {
		t.Fatalf("got %v", got)
	}
}

func TestSessionsScopeSkillsRecentFirst(t *testing.T) {
	in := palInput{Query: "x", Recent: []string{"/xb", "/gone"}, Skills: []skills.Skill{{Name: "xc"}, {Name: "xA"}, {Name: "xb"}}}
	got := buildPalette(in)
	if strings.Join(palNames(got), ",") != "/xb,/xA,/xc" {
		t.Fatalf("got %v", palNames(got))
	}
	if got[0].Prompt != "/xb" || got[0].Key != "skill:xb" {
		t.Fatalf("item %+v", got[0])
	}
}

func TestSessionsKeepSidebarOrder(t *testing.T) {
	in := palInput{Query: "o", Sessions: []sideRow{{ID: 3, Title: "zoo"}, {ID: 1, Title: "foo"}, {ID: 2, Title: "boo"}}}
	if got := palNames(buildPalette(in)); strings.Join(got, ",") != "zoo,foo,boo" {
		t.Fatalf("got %v", got)
	}
}

func TestRunWorkspaceItems(t *testing.T) {
	acts := workspaceActions([]workspaces.Workspace{{ID: "samurai", Name: "samurai"}})
	got := buildPalette(palInput{Query: "samurai", Actions: acts})
	if len(got) != 1 || got[0].Name != "Run workspace samurai" || got[0].Key != "ws:samurai" || got[0].Workspace != "samurai" || got[0].Icon != "workspaces" {
		t.Fatalf("got %+v", got)
	}
}

func TestSplitShortcut(t *testing.T) {
	for in, want := range map[string]string{"": "[]", "Ctrl+Shift+T": "[Ctrl Shift T]", "Ctrl++": "[Ctrl +]", "F5": "[F5]"} {
		if got := fmt.Sprint(splitShortcut(in)); got != want {
			t.Errorf("%q: %s, want %s", in, got, want)
		}
	}
}

func TestPaletteNoMatches(t *testing.T) {
	for _, q := range []string{"zzz", "/zzz"} {
		if got := buildPalette(withQuery(q)); len(got) != 0 {
			t.Fatalf("%s: got %v", q, palNames(got))
		}
	}
}

func TestRecentKeepsTenDistinct(t *testing.T) {
	var r recentPrompts
	for i := 0; i < 12; i++ {
		r.add(fmt.Sprintf("/p%d", i))
	}
	r.add("/p5")
	if len(r.list) != maxRecent || r.list[0] != "/p5" {
		t.Fatalf("list %v", r.list)
	}
	seen := map[string]bool{}
	for _, p := range r.list {
		if seen[p] {
			t.Fatalf("duplicate %s in %v", p, r.list)
		}
		seen[p] = true
	}
}

func TestSessionRowsGroupsThenLoose(t *testing.T) {
	m := sideModel{
		Groups: []sideGroup{{Rows: []sideRow{{ID: 2}, {ID: 1}}}, {Rows: []sideRow{{ID: 5}}}},
		Loose:  []sideRow{{ID: 3}},
	}
	var ids []int
	for _, r := range sessionRows(m) {
		ids = append(ids, int(r.ID))
	}
	if fmt.Sprint(ids) != "[2 1 5 3]" {
		t.Fatalf("got %v", ids)
	}
}

func groupHeads(gs []palGroup) []string {
	var out []string
	for _, g := range gs {
		out = append(out, g.Head)
	}
	return out
}

func skillNames(list []skills.Skill) []string {
	var out []string
	for _, s := range list {
		out = append(out, s.Name)
	}
	return out
}

func TestGroupSkillsOrder(t *testing.T) {
	gs := groupSkills([]skills.Skill{
		{Name: "superpowers:b", Scope: skills.Plugin},
		{Name: "zeta", Scope: skills.User},
		{Name: "devbox:run", Scope: skills.Plugin},
		{Name: "local", Scope: skills.Project},
		{Name: "Devbox:kit", Scope: skills.Plugin},
		{Name: "alpha", Scope: skills.User},
		{Name: "devbox:dash", Scope: skills.Plugin},
	})
	// Ruling 4: "Devbox" and "devbox" are two plugins, ordered next to each other.
	want := []string{"YOURS", "THIS PROJECT", "DEVBOX", "DEVBOX", "SUPERPOWERS"}
	if fmt.Sprint(groupHeads(gs)) != fmt.Sprint(want) {
		t.Fatalf("heads %v, want %v", groupHeads(gs), want)
	}
	if got := skillNames(gs[0].Skills); fmt.Sprint(got) != "[alpha zeta]" || gs[0].Plugin != "" {
		t.Fatalf("yours %v plugin %q", got, gs[0].Plugin)
	}
	if gs[1].Plugin != "" || fmt.Sprint(skillNames(gs[1].Skills)) != "[local]" {
		t.Fatalf("project %+v", gs[1])
	}
	if gs[2].Plugin != "Devbox" || fmt.Sprint(skillNames(gs[2].Skills)) != "[Devbox:kit]" {
		t.Fatalf("Devbox group %+v", gs[2])
	}
	if gs[3].Plugin != "devbox" || fmt.Sprint(skillNames(gs[3].Skills)) != "[devbox:dash devbox:run]" {
		t.Fatalf("devbox group %+v", gs[3])
	}
	if gs[4].Plugin != "superpowers" {
		t.Fatalf("superpowers group %+v", gs[4])
	}
}

func TestGroupSkillsOddNames(t *testing.T) {
	gs := groupSkills([]skills.Skill{
		{Name: "orphan", Scope: skills.Plugin},
		{Name: "team:review", Scope: skills.User},
		{Name: "a:b:c", Scope: skills.Plugin},
		{Name: ":x", Scope: skills.Plugin},
	})
	if fmt.Sprint(groupHeads(gs)) != "[YOURS A OTHER]" {
		t.Fatalf("heads %v", groupHeads(gs))
	}
	if fmt.Sprint(skillNames(gs[0].Skills)) != "[team:review]" {
		t.Fatalf("yours %v", skillNames(gs[0].Skills))
	}
	if gs[1].Plugin != "a" || fmt.Sprint(skillNames(gs[1].Skills)) != "[a:b:c]" {
		t.Fatalf("a %+v", gs[1])
	}
	if gs[2].Plugin != "" || fmt.Sprint(skillNames(gs[2].Skills)) != "[:x orphan]" {
		t.Fatalf("other %+v", gs[2])
	}
	if pluginOf(skills.Skill{Name: "team:review", Scope: skills.User}) != "" || pluginOf(skills.Skill{Name: "a:b:c", Scope: skills.Plugin}) != "a" {
		t.Fatal("pluginOf")
	}
}

func TestGroupSkillsNoProject(t *testing.T) {
	gs := groupSkills([]skills.Skill{{Name: "x", Scope: skills.User}, {Name: "p:y", Scope: skills.Plugin}})
	if fmt.Sprint(groupHeads(gs)) != "[YOURS P]" {
		t.Fatalf("heads %v", groupHeads(gs))
	}
	if gs := groupSkills(nil); gs != nil {
		t.Fatalf("nil list: %v", gs)
	}
}

func TestRecentSkillsCappedAndPresent(t *testing.T) {
	list := []skills.Skill{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}}
	got := recentSkills(list, []string{"/gone", "/c", "/a", "/b", "/d"})
	if fmt.Sprint(skillNames(got)) != "[c a b]" {
		t.Fatalf("recent %v", skillNames(got))
	}
	if palRecentMax != 3 || palFoldAt != 4 {
		t.Fatal("constants")
	}
}

// groupSkills runs once per scan and builds each skill's row then, so the
// frames do not: two strings per skill ("skill:<name>", "/<name>") and a few
// per group, nothing that grows faster.
func TestGroupSkillsAllocsBounded(t *testing.T) {
	var list []skills.Skill
	for i := range skills.MaxSkills {
		s := skills.Skill{Name: fmt.Sprintf("p%d:skill-%04d", i%10, (i*7919)%skills.MaxSkills), Scope: skills.Plugin}
		switch i % 5 {
		case 0:
			s.Name, s.Scope = fmt.Sprintf("Mine-%04d", i), skills.User
		case 1:
			s.Name, s.Scope = fmt.Sprintf("proj-%04d", i), skills.Project
		}
		list = append(list, s)
	}
	gs := groupSkills(list)
	n := 0
	for _, g := range gs {
		n += len(g.Skills)
		for i := 1; i < len(g.Skills); i++ {
			if strings.ToLower(g.Skills[i-1].Name) > strings.ToLower(g.Skills[i].Name) {
				t.Fatalf("%s not sorted at %d", g.Head, i)
			}
		}
	}
	if n != len(list) {
		t.Fatalf("%d skills grouped of %d", n, len(list))
	}
	allocs := testing.AllocsPerRun(5, func() { groupSkills(list) })
	if limit := float64(2*len(list) + 4*len(gs) + 8); allocs > limit {
		t.Fatalf("%.0f allocs for %d skills in %d groups, want at most %.0f", allocs, len(list), len(gs), limit)
	}
}

// palHeaders are the headers drawn above items: where a group starts.
func palHeaders(items []palItem) []string {
	var out []string
	for i, it := range items {
		if firstInGroup(items, i) == i {
			out = append(out, it.Group)
		}
	}
	return out
}

// A plugin named "Other" and the Other group are two groups, and so are two
// spellings of one plugin: each folds, opens and gets its header on its own.
func TestPluginGroupsNeverShareAKey(t *testing.T) {
	var list []skills.Skill
	add := func(prefix string, scope skills.Scope) {
		for i := range 6 {
			list = append(list, skills.Skill{Name: fmt.Sprintf("%sskill-%d", prefix, i), Scope: scope})
		}
	}
	add("Other:", skills.Plugin)
	add("orphan-", skills.Plugin) // no plugin name: the Other group
	add("Devbox:", skills.Plugin)
	add("devbox:", skills.Plugin)
	in := palInput{Query: "/", Skills: list, Groups: groupSkills(list)}
	got := buildPalette(in)
	if h := palHeaders(got); fmt.Sprint(h) != "[DEVBOX DEVBOX OTHER OTHER]" {
		t.Fatalf("headers %v", h)
	}
	keys := map[string]bool{}
	for _, it := range got {
		if keys[it.Key] {
			t.Fatalf("key %s twice in %v", it.Key, palNames(got))
		}
		keys[it.Key] = true
	}
	var mores []palItem
	for _, it := range got {
		if it.Kind == palMoreGroup {
			mores = append(mores, it)
		}
	}
	if len(mores) != 4 {
		t.Fatalf("%d more rows, want 4", len(mores))
	}
	for _, m := range mores {
		in.Open = map[string]bool{m.Fold: true}
		opened := buildPalette(in)
		n := 0
		for _, it := range opened {
			if it.Kind == palMoreGroup {
				n++
			}
		}
		if n != 3 {
			t.Fatalf("opening %q (%s) left %d more rows, want 3", m.Fold, m.Name, n)
		}
	}
}
