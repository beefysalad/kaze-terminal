package ui

import (
	"image"
	"io"
	"runtime"
	"slices"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// cmdLine is one line of Settings > Commands' command-line list; Copy "" has no button.
type cmdLine struct{ Example, Copy, Desc string }

var commandLines = []cmdLine{
	{"kaze --workspace <name>", `kaze --workspace "<name>"`, "Start kaze with a workspace"},
	{"kaze --version", "kaze --version", "Print the version and build"},
	{"kaze notify done / needs / prompt / interrupt", "", "Used by Claude Code and Codex hooks; you don't run these yourself"},
	{"kaze statusline", "", "Used by Claude Code's status line; you don't run this yourself"},
}

// mouseLines are selection gestures listed with the shortcuts; they have no key to change.
var mouseLines = [][2]string{
	{"Select a word / a line", "double / triple click"},
	{"Select in mouse apps", "Shift + drag"},
}

// shortcutGroups sorts kaze's actions into the page's cards; an action not
// listed lands in Other.
var shortcutGroups = []struct {
	title   string
	configs []string
}{
	{"Sessions", []string{"new_tab", "new_branch_session", "close_tab", "next_tab", "prev_tab", "next_waiting"}},
	{"Kaze", []string{"palette", "toggle_sidebar", "open_settings"}},
	{"Editing and mouse", []string{"copy", "paste", "find"}},
}

func commandsSection() settingsSection {
	return settingsSection{
		title:  "Shortcuts",
		hint:   keysHint,
		draw:   drawCommands,
		act:    commandsAct,
		show:   (*window).checkKeys,
		dialog: keysDialogBox,
	}
}

func commandsAct(win *window, id string, arg int) {
	if id == "cmd_copy" && arg >= 0 && arg < len(commandLines) {
		win.set.cmdCopy = commandLines[arg].Copy
	}
	if strings.HasPrefix(id, "key") {
		win.keysAct(id, arg)
	}
}

// keysAct runs the Shortcuts page's own controls; arg is an editableActions index.
func (win *window) keysAct(id string, arg int) {
	switch id {
	case "key_reset_all":
		win.stopRecording()
		win.set.keysConfirm = win.set.keysErr == ""
		return
	case "keys_cancel":
		win.set.keysConfirm = false
		return
	case "keys_reset_all":
		win.resetAllKeys()
		return
	}
	names := editableActions(runtime.GOOS)
	if arg < 0 || arg >= len(names) {
		return
	}
	switch id {
	case "key_rec":
		win.startRecording(names[arg])
	case "key_reset":
		win.resetKey(names[arg])
	case "key_ok", "key_cancel", "key_replace":
		win.keysStrip(id)
	}
}

// shortcutRow is one line of a shortcuts card; idx is the action's
// editableActions index, -1 for a mouse gesture.
type shortcutRow struct {
	label, keys, config string
	idx                 int
}

// drawCommands lists kaze's shortcuts as this machine has them, overrides
// included, in grouped cards with key caps, then kaze's command line.
func drawCommands(p *page, r image.Rectangle, win *window) int {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	if text := win.set.cmdCopy; text != "" {
		win.set.cmdCopy = ""
		if win.toClipboard != nil {
			win.toClipboard(text)
		} else {
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
		}
		win.note("Copied: " + text)
	}
	sec := commandsSection()
	hint := keysHint
	if win.set.keysErr != "" {
		hint = win.set.keysErr
	}
	lines := p.wrap(hint, 13, max(r.Dx(), 1)) // under the title, clear of the search field
	y := p.heading(r, sec.title, lines[0])
	for _, l := range lines[1:] {
		call, size := p.text(l, 13, col.muted)
		place(ops, call, image.Pt(r.Min.X, y-gtx.Dp(22)))
		y += size.Y
	}
	p.headerActions(r, []ctrl{{kind: ctlText, id: "cmd_filter", val: win.set.cmdFilter, placeholder: "Search shortcuts", width: 220,
		set: func(s string) { win.set.cmdFilter = s }}})

	q := strings.ToLower(strings.TrimSpace(win.set.cmdFilter))
	match := func(r shortcutRow) bool {
		return q == "" || strings.Contains(strings.ToLower(r.label+" "+r.keys+" "+r.config), q)
	}
	keysOf := win.keyText // "" for an action without a shortcut
	editable := editableActions(runtime.GOOS)
	row := func(c kazeCommand) shortcutRow {
		return shortcutRow{c.Label, keysOf(c.Config), c.Config, slices.Index(editable, c.Config)}
	}
	grouped := map[string]bool{}
	var groups [][]shortcutRow
	var titles []string
	for _, g := range shortcutGroups {
		var rows []shortcutRow
		for _, cfg := range g.configs {
			grouped[cfg] = true
			for _, c := range kazeCommands {
				if c.Config == cfg {
					rows = append(rows, row(c))
				}
			}
		}
		if g.title == "Editing and mouse" {
			for _, m := range mouseLines {
				rows = append(rows, shortcutRow{label: m[0], keys: m[1], idx: -1})
			}
		}
		groups, titles = append(groups, rows), append(titles, g.title)
	}
	var other []shortcutRow
	for _, c := range kazeCommands {
		if !grouped[c.Config] {
			other = append(other, row(c))
		}
	}
	if len(other) > 0 {
		groups, titles = append(groups, other), append(titles, "Other")
	}

	shown := false
	for gi, rows := range groups {
		var keep []shortcutRow
		for _, row := range rows {
			if match(row) {
				keep = append(keep, row)
			}
		}
		if len(keep) == 0 {
			continue
		}
		if shown {
			y += gtx.Dp(26)
		}
		y = p.sectionLabel(r, y, titles[gi])
		y = p.shortcutCard(r, y, keep)
		shown = true
	}
	if !shown {
		call, size := p.text("No shortcut matches \""+win.set.cmdFilter+"\".", 13, col.muted)
		place(ops, call, image.Pt(r.Min.X+gtx.Dp(2), y))
		y += size.Y
	}
	if win.set.keysErr == "" {
		y = p.resetAllLink(r, y+gtx.Dp(6))
	}

	y += gtx.Dp(26)
	y = p.sectionLabel(r, y, "Command line")
	rec := op.Record(ops)
	top := y
	pad := gtx.Dp(18)
	for i, l := range commandLines {
		if i > 0 {
			paint.FillShape(ops, col.line, clip.Rect(image.Rect(r.Min.X, y, r.Max.X, y+1)).Op())
		}
		ec, es := p.mono(l.Example, 12.5, col.title)
		dc, ds := p.text(l.Desc, 12, col.muted)
		h := max(es.Y+ds.Y+gtx.Dp(26), gtx.Dp(58))
		ty := y + (h-es.Y-ds.Y)/2
		place(ops, ec, image.Pt(r.Min.X+pad, ty))
		place(ops, dc, image.Pt(r.Min.X+pad, ty+es.Y+gtx.Dp(2)))
		if l.Copy != "" {
			call, sz, from := p.ctrl(ctrl{kind: ctlButton, id: "cmd_copy", arg: i, text: "Copy", ghost: true}, 0)
			at := image.Pt(r.Max.X-pad-sz.X, y+(h-sz.Y)/2)
			p.st.shift(from, len(p.st.controls), at)
			place(ops, call, at)
		}
		y += h
	}
	call := rec.Stop()
	card := image.Rect(r.Min.X, top, r.Max.X, y)
	paint.FillShape(ops, col.card, clip.UniformRRect(card, gtx.Dp(12)).Op(ops))
	call.Add(ops)
	stroke(ops, card, gtx.Dp(12), col.line)
	p.st.cards = append(p.st.cards, card)
	return y + gtx.Dp(16)
}

// resetAllLink draws "Reset all to defaults" right-aligned from y and returns
// where the next thing starts.
func (p *page) resetAllLink(r image.Rectangle, y int) int {
	gtx, ops, col := p.gtx, p.gtx.Ops, p.col
	call, size := p.text("Reset all to defaults", 13, col.muted)
	at := image.Pt(r.Max.X-size.X-gtx.Dp(2), y)
	place(ops, call, at)
	u := at.Y + size.Y - gtx.Dp(1)
	paint.FillShape(ops, col.muted, clip.Rect(image.Rect(at.X, u, at.X+size.X, u+max(gtx.Dp(1), 1))).Op())
	hit := image.Rectangle{Min: at, Max: at.Add(size)}.Inset(-gtx.Dp(3))
	p.hit(ctlButton, hit, "key_reset_all", 0, false)
	return hit.Max.Y
}

// recordingText is what a recording row's button reads.
const recordingText = "Press the new keys… Esc cancels"
