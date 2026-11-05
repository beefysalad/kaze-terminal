package ui

import (
	"cmp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/beefysalad/kaze-terminal/internal/skills"
	"github.com/beefysalad/kaze-terminal/internal/tabs"
	"github.com/beefysalad/kaze-terminal/internal/workspaces"
)

// palKind is what a palette row is for.
type palKind int

const (
	palSession     palKind = iota // a session: go there
	palSkill                      // a skill: type it here, Shift: a new session
	palKaze                       // a Kaze action or "Run workspace <name>"
	palBrowse                     // "Browse all N skills": switch to Skills
	palMoreMatches                // "N more matching skills": switch to Skills with the text
	palMoreGroup                  // "N more in <plugin>": open the group in place
)

// The groups' headers, drawn upper case.
const (
	palHeadWaiting      = "WAITING ON YOU"
	palHeadSessions     = "SESSIONS"
	palHeadRecentSkills = "RECENT SKILLS"
	palHeadKaze         = "KAZE"
	palHeadSkills       = "SKILLS"
	palHeadRecent       = "RECENT"
)

// palOtherKey names the Other group in Open and in its "more" row.
const palOtherKey = "Other"

// palItem is one palette row.
type palItem struct {
	Kind       palKind
	Key        string // stable across rebuilds: "tab:<id>", "skill:<name>", "recent:<name>", "act:<config>", "ws:<id>", "browse", "more:matches", "more:<plugin>"
	Group      string // header text above it (upper case); a header is drawn where it changes
	GroupCount string // count right of the header, "" for none
	Plugin     string // plugin group: tile initial and colour; palMoreGroup: which group to open
	Name       string // title, action name, "/plugin:skill", or the row text for browse and more
	Dim        int    // bytes of Name drawn dim: len("/devbox:") for a plugin skill, else 0
	Agent      string // badge: "claude", "codex"; "" none
	State      string // pill: "needs you", "running", "idle", "done", "shell"
	Detail     string // skill description
	Keys       []string
	Icon       string // action tile icon (iconNames), "" none
	Mark       mark
	Running    bool
	Tab        tabs.TabID
	Prompt     string // "/" + skill name
	Act        action
	Workspace  string // workspace id for "Run workspace <name>"
	Count      int    // browse and more rows: the N
	Query      string // palMoreMatches: the text to switch to
}

// palAction is one Kaze action the palette can run.
type palAction struct {
	Name, Config, Workspace, Icon string
	Keys                          []string // formatShortcut split at "+"; nil for none
	Act                           action
}

type palInput struct {
	Query    string
	Sessions []sideRow      // sidebar order, unfiltered
	Skills   []skills.Skill // the catalog, for counts and Sessions-scope matches
	Groups   []palGroup     // groupSkills(Skills), cached by the window
	Recent   []string       // prompts, most recent first
	Actions  []palAction
	Open     map[string]bool // plugin groups opened by "N more in <plugin>"
}

// palSkillScope: text starting with "/" (after spaces) is the Skills scope.
func palSkillScope(q string) bool { return strings.HasPrefix(strings.TrimLeft(q, " "), "/") }

// splitShortcut splits formatShortcut's text into keycaps: "Ctrl+Shift+T" is
// Ctrl, Shift, T; a "+" key stays one cap.
func splitShortcut(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "+")
	if n := len(parts); n >= 2 && parts[n-1] == "" && parts[n-2] == "" {
		parts = append(parts[:n-2], "+")
	}
	return parts
}

// palQuery is the query as matched: lowercased, trimmed, one leading "/" dropped.
func palQuery(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	return strings.TrimPrefix(q, "/")
}

// palTier ranks a match inside its group, lower first: the name starts with
// q (for "plugin:name" any part after a ":" too), the name contains q, only
// the other text contains q. ok is false for no match.
func palTier(q, name string, other ...string) (tier int, ok bool) {
	if q == "" {
		return 0, true
	}
	n := strings.ToLower(name)
	if strings.HasPrefix(n, q) {
		return 0, true
	}
	for i := strings.IndexByte(n, ':'); i >= 0; {
		n2 := n[i+1:]
		if strings.HasPrefix(n2, q) {
			return 0, true
		}
		j := strings.IndexByte(n2, ':')
		if j < 0 {
			break
		}
		i += 1 + j
	}
	if strings.Contains(n, q) {
		return 1, true
	}
	for _, o := range other {
		if strings.Contains(strings.ToLower(o), q) {
			return 2, true
		}
	}
	return 0, false
}

type ranked struct {
	palItem
	tier int
}

// byTier orders matches by tier, keeping their base order inside a tier.
func byTier(rs []ranked) {
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].tier < rs[j].tier })
}

// buildPalette lists the rows for in.Query. Text starting with "/" is the
// Skills scope (grouped skills); anything else is the Sessions scope
// (sessions, then on text Kaze actions and a few skills).
func buildPalette(in palInput) []palItem {
	q := palQuery(in.Query)
	if palSkillScope(in.Query) {
		return buildSkillScope(in, q)
	}
	return buildSessionScope(in, q)
}

func buildSessionScope(in palInput, q string) []palItem {
	var waiting, sessions []ranked
	for _, r := range in.Sessions {
		tier, ok := palTier(q, r.Title, r.Command, r.Folder, r.Branch, r.Agent)
		if !ok {
			continue
		}
		it := sessionItem(r)
		if r.Mark == markNeeds {
			waiting = append(waiting, ranked{it, tier})
		} else {
			sessions = append(sessions, ranked{it, tier})
		}
	}
	var out []palItem
	for _, sec := range [][]ranked{waiting, sessions} {
		byTier(sec)
		for _, r := range sec {
			out = append(out, r.palItem)
		}
	}
	if q == "" {
		rec := recentSkills(in.Skills, in.Recent)
		for _, s := range rec {
			it := skillItem(s, palHeadRecentSkills)
			it.Key = "recent:" + s.Name
			out = append(out, it)
		}
		if n := len(in.Skills); n > 0 {
			group := palHeadRecentSkills
			if len(rec) == 0 {
				group = palHeadSkills
			}
			out = append(out, palItem{Kind: palBrowse, Key: "browse", Group: group, Name: "Browse all " + strconv.Itoa(n) + " skills", Count: n})
		}
		return out
	}
	var kaze []ranked
	for _, a := range in.Actions {
		tier, ok := palTier(q, a.Name, strings.Join(a.Keys, "+"))
		if !ok {
			continue
		}
		it := palItem{Kind: palKaze, Key: "act:" + a.Config, Group: palHeadKaze, Name: a.Name, Keys: a.Keys, Icon: a.Icon, Act: a.Act, Workspace: a.Workspace}
		if a.Workspace != "" {
			it.Key = "ws:" + a.Workspace
		}
		kaze = append(kaze, ranked{it, tier})
	}
	byTier(kaze)
	for _, r := range kaze {
		out = append(out, r.palItem)
	}
	var skl []ranked
	for _, s := range orderSkills(in.Skills, in.Recent) {
		if tier, ok := palTier(q, s.Name, s.Desc); ok {
			skl = append(skl, ranked{skillItem(s, palHeadSkills), tier})
		}
	}
	byTier(skl)
	count := strconv.Itoa(len(skl))
	for _, r := range skl[:min(len(skl), palRecentMax)] {
		r.GroupCount = count
		out = append(out, r.palItem)
	}
	if n := len(skl) - palRecentMax; n > 0 {
		out = append(out, palItem{
			Kind: palMoreMatches, Key: "more:matches", Group: palHeadSkills, GroupCount: count,
			Name: strconv.Itoa(n) + " more matching skills", Count: n, Query: "/" + strings.TrimSpace(in.Query),
		})
	}
	return out
}

func buildSkillScope(in palInput, q string) []palItem {
	var out []palItem
	if q == "" {
		for _, s := range recentSkills(in.Skills, in.Recent) {
			it := skillItem(s, palHeadRecent)
			it.Key = "recent:" + s.Name
			out = append(out, it)
		}
	}
	var matches []ranked
	for _, g := range in.Groups {
		matches = matches[:0]
		for _, s := range g.Skills {
			if tier, ok := palTier(q, s.Name, s.Desc); ok {
				matches = append(matches, ranked{skillItem(s, g.Head), tier})
			}
		}
		if len(matches) == 0 {
			continue
		}
		byTier(matches)
		count := strconv.Itoa(len(matches))
		key := palGroupKey(g)
		folds := key != "" // plugin groups and Other; Yours and This project never fold
		shown := len(matches)
		if q == "" && folds && !in.Open[key] {
			shown = min(shown, palFoldAt)
		}
		for _, r := range matches[:shown] {
			r.GroupCount = count
			out = append(out, r.palItem)
		}
		if n := len(matches) - shown; n > 0 {
			out = append(out, palItem{
				Kind: palMoreGroup, Key: "more:" + key, Group: g.Head, GroupCount: count, Plugin: key,
				Name: strconv.Itoa(n) + " more in " + key, Count: n,
			})
		}
	}
	return out
}

func sessionItem(r sideRow) palItem {
	it := palItem{
		Kind: palSession, Key: "tab:" + strconv.Itoa(int(r.ID)), Group: palHeadSessions, Name: r.Title,
		Agent: r.Agent, State: sessionWord(r), Mark: r.Mark, Running: r.Running, Tab: r.ID,
	}
	if it.Agent == "" && r.Claude {
		it.Agent = "claude"
	}
	if r.Mark == markNeeds {
		it.Group, it.State = palHeadWaiting, "needs you"
	}
	return it
}

func skillItem(s skills.Skill, group string) palItem {
	it := palItem{Kind: palSkill, Key: "skill:" + s.Name, Group: group, Name: "/" + s.Name, Detail: s.Desc, Prompt: "/" + s.Name}
	if p := pluginOf(s); p != "" {
		it.Plugin, it.Dim = p, len(p)+2
	}
	return it
}

// sessionWord is a session's state in the palette: done, running, idle for a
// Claude session at rest, shell otherwise.
func sessionWord(r sideRow) string {
	switch {
	case r.Mark == markDone:
		return "done"
	case r.Running:
		return "running"
	case r.Claude || r.Agent != "":
		return "idle"
	}
	return "shell"
}

// orderSkills puts recently started skills first, most recent first, then the
// rest by name, case-insensitively.
func orderSkills(list []skills.Skill, recent []string) []skills.Skill {
	pos := map[string]int{}
	for i, p := range recent {
		pos[strings.TrimPrefix(p, "/")] = i + 1
	}
	out := append([]skills.Skill(nil), list...)
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := pos[out[i].Name], pos[out[j].Name]
		if (pi > 0) != (pj > 0) {
			return pi > 0
		}
		if pi > 0 {
			return pi < pj
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// workspaceActions are the Kaze actions "Run workspace <name>" items.
func workspaceActions(spaces []workspaces.Workspace) []palAction {
	var out []palAction
	for _, ws := range spaces {
		name := strings.TrimSpace(ws.Name)
		if name == "" {
			name = ws.ID
		}
		out = append(out, palAction{Name: "Run workspace " + name, Workspace: ws.ID, Icon: "workspaces"})
	}
	return out
}

const maxRecent = 10

// recentPrompts are the last prompts started from the palette, most recent
// first, in memory only.
type recentPrompts struct{ list []string }

func (r *recentPrompts) add(p string) {
	out := []string{p}
	for _, q := range r.list {
		if q != p && len(out) < maxRecent {
			out = append(out, q)
		}
	}
	r.list = out
}

const (
	palRecentMax = 3 // recent skills shown
	palFoldAt    = 4 // skills shown per folded plugin group
)

const (
	palHeadYours   = "YOURS"
	palHeadProject = "THIS PROJECT"
	palHeadOther   = "OTHER"
)

// palGroup is one source of skills in the Skills scope.
type palGroup struct {
	Head   string         // drawn header: "YOURS", "THIS PROJECT", "DEVBOX", "OTHER"
	Plugin string         // plugin groups only: "devbox"; "" for Yours, This project, Other
	Skills []skills.Skill // sorted by name, case-insensitively
}

// pluginOf is a plugin skill's plugin: the text before its first ":"; "" for
// any other scope, or a plugin skill without one.
func pluginOf(s skills.Skill) string {
	if s.Scope != skills.Plugin {
		return ""
	}
	if i := strings.IndexByte(s.Name, ':'); i > 0 {
		return s.Name[:i]
	}
	return ""
}

// groupRank orders the groups: Yours, This project, plugins, Other.
func groupRank(s skills.Skill) int {
	switch s.Scope {
	case skills.User:
		return 0
	case skills.Project:
		return 1
	case skills.Plugin:
		if pluginOf(s) != "" {
			return 2
		}
	}
	return 3
}

// foldCompare compares case-insensitively without allocating.
func foldCompare(a, b string) int {
	for a != "" && b != "" {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		la, lb := unicode.ToLower(ra), unicode.ToLower(rb)
		if la != lb {
			return cmp.Compare(la, lb)
		}
		a, b = a[na:], b[nb:]
	}
	return cmp.Compare(len(a), len(b))
}

// groupSkills splits list by where each skill comes from: Yours, This project,
// one group per plugin A to Z (case-insensitively; two spellings are two
// plugins), then Other. Empty groups are left out. The groups share one
// sorted copy of list.
func groupSkills(list []skills.Skill) []palGroup {
	if len(list) == 0 {
		return nil
	}
	sorted := slices.Clone(list)
	slices.SortStableFunc(sorted, func(a, b skills.Skill) int {
		if c := cmp.Compare(groupRank(a), groupRank(b)); c != 0 {
			return c
		}
		pa, pb := pluginOf(a), pluginOf(b)
		if c := foldCompare(pa, pb); c != 0 {
			return c
		}
		if c := strings.Compare(pa, pb); c != 0 {
			return c
		}
		return foldCompare(a.Name, b.Name)
	})
	var out []palGroup
	for start := 0; start < len(sorted); {
		rank, plugin := groupRank(sorted[start]), pluginOf(sorted[start])
		end := start + 1
		for end < len(sorted) && groupRank(sorted[end]) == rank && pluginOf(sorted[end]) == plugin {
			end++
		}
		g := palGroup{Plugin: plugin, Skills: sorted[start:end:end]}
		switch rank {
		case 0:
			g.Head = palHeadYours
		case 1:
			g.Head = palHeadProject
		case 2:
			g.Head = strings.ToUpper(plugin)
		default:
			g.Head = palHeadOther
		}
		out = append(out, g)
		start = end
	}
	return out
}

// recentSkills are the recent prompts still in list, most recent first, at
// most palRecentMax.
func recentSkills(list []skills.Skill, recent []string) []skills.Skill {
	var out []skills.Skill
	for _, p := range recent {
		if len(out) == palRecentMax {
			break
		}
		name := strings.TrimPrefix(p, "/")
		for _, s := range list {
			if s.Name == name {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// palGroupKey names a group in palInput.Open and in its "more" row: the
// plugin, or "Other"; "" for Yours and This project, which never fold.
func palGroupKey(g palGroup) string {
	if g.Head == palHeadOther {
		return palOtherKey
	}
	return g.Plugin
}
