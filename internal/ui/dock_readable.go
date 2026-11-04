package ui

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/beefysalad/kaze-terminal/internal/subagents"
)

// The readable Activity (issue #134): the agent's words, then each step as a
// short sentence, runs of small steps folded. Built once per change (see
// dockSession.readable), never per frame.

type readKind uint8

const (
	readNote readKind = iota
	readStep
	readDropped
	readWaiting
	readLive
	readFinished
)

// Meta tones: plain, an added count, a removed count, passed, failed.
const (
	tonePlain uint8 = iota
	toneAdd
	toneDel
	toneOK
	toneBad
)

const (
	readDetailLines = 24  // lines a fold or a joined diff keeps
	readCmdRunes    = 120 // a command shown as a step's object
	readCapRunes    = 200 // the "$ command" caption of a detail
)

type readMeta struct {
	text string
	tone uint8
}

// readDetail is what a click opens: an optional "$ command" caption and
// lines. Line kinds: '+' and '-' diff lines, ' ' dim (context, output),
// 'l' a plain list line (a fold's files or searches).
type readDetail struct {
	cap   string
	lines []subagents.DiffLine
}

type readItem struct {
	kind    readKind
	id      string // the first step's ID: the open state survives trimming
	at      time.Time
	verb    string // "Edited"
	obj     string // "internal/ui/agent.go"
	objMono bool
	meta    [2]readMeta // fixed so an item needs no slice of its own
	nmeta   uint8
	detail  readDetail
	text    string // notes and the end rows
	msg     int    // a note's index in the transcript's Messages, else -1
	clock   string // a note's time, "15:04", set when the list is built
}

func (it *readItem) metas() []readMeta { return it.meta[:it.nmeta] }
func (it *readItem) hasDetail() bool   { return it.detail.cap != "" || len(it.detail.lines) > 0 }
func (it *readItem) addMeta(s string, tone uint8) {
	if int(it.nmeta) < len(it.meta) {
		it.meta[it.nmeta] = readMeta{s, tone}
		it.nmeta++
	}
}

// plumbing tools are hidden in the readable view; they neither show nor end a run.
func plumbing(tool string) bool {
	switch tool {
	case "ToolSearch", "Monitor", "TaskStop", "TaskOutput", "BashOutput", "KillShell":
		return true
	}
	return false
}

type foldClass uint8

const (
	foldNone foldClass = iota
	foldReads
	foldSearches
	foldEdits
	foldTodos
)

func foldOf(tool string) foldClass {
	switch tool {
	case "Read":
		return foldReads
	case "Grep", "Glob":
		return foldSearches
	case "Edit", "Write", "MultiEdit", "NotebookEdit", "apply_patch":
		return foldEdits
	case "TodoWrite":
		return foldTodos
	}
	return foldNone
}

// pastVerbs turns the imperative a Bash description starts with into the
// past tense; keyed lower case.
var pastVerbs = map[string]string{
	"run": "Ran", "check": "Checked", "list": "Listed", "find": "Found", "build": "Built",
	"show": "Showed", "create": "Created", "install": "Installed", "remove": "Removed",
	"delete": "Deleted", "update": "Updated", "fetch": "Fetched", "read": "Read",
	"search": "Searched", "get": "Got", "make": "Made", "start": "Started", "stop": "Stopped",
	"test": "Tested", "print": "Printed", "count": "Counted", "copy": "Copied", "move": "Moved",
	"write": "Wrote", "open": "Opened", "merge": "Merged", "push": "Pushed", "commit": "Committed",
}

// testCommand reports whether a shell command runs a test suite, matched on
// its words: a runner (jest, pytest, vitest, also as a path's last part) or a
// tool's test command (go test, cargo test, npm test, npm run test, ...). A
// word that only contains one ("jest.config.js") does not count.
func testCommand(cmd string) bool {
	words := strings.FieldsFunc(cmd, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(";&|()", r)
	})
	for i, w := range words {
		switch w[strings.LastIndexAny(w, `/\`)+1:] {
		case "jest", "pytest", "vitest":
			return true
		case "go", "cargo", "npm", "pnpm", "yarn", "bun":
			next := ""
			if i+1 < len(words) {
				next = words[i+1]
			}
			if next == "run" && w != "go" && w != "cargo" && i+2 < len(words) {
				next = words[i+2]
			}
			if next == "test" {
				return true
			}
		}
	}
	return false
}

// rdEntry is one source row before folding: a note or a step. hook marks a
// step that came from the hook summaries (no results, counts or diffs).
type rdEntry struct {
	note bool
	text string
	msg  int
	at   time.Time
	step *subagents.TranscriptStep
}

// readableItems is a's readable Activity. The transcript's steps and notes
// interleave by record order; without transcript steps (Codex, or a
// transcript Kaze never learned) the hook steps and the notes go by time.
func readableItems(a *subagents.Agent, tr *subagents.Transcript) []readItem {
	return appendReadable(nil, a, tr)
}

// appendReadable appends a's readable Activity to out (out[:0] to reuse).
func appendReadable(out []readItem, a *subagents.Agent, tr *subagents.Transcript) []readItem {
	var ents []rdEntry
	hook := !(tr != nil && tr.Available && len(tr.Steps) > 0)
	if !hook {
		if tr.StepsDropped {
			out = append(out, readItem{kind: readDropped, msg: -1, text: "… earlier steps hidden"})
		}
		ents = make([]rdEntry, 0, len(tr.Steps)+len(tr.Messages))
		m, s := 0, 0
		for m < len(tr.Messages) || s < len(tr.Steps) {
			if s < len(tr.Steps) && (m == len(tr.Messages) || tr.Steps[s].Seq < tr.Messages[m].Seq) {
				ents = append(ents, rdEntry{step: &tr.Steps[s], at: tr.Steps[s].At})
				s++
				continue
			}
			if msg := tr.Messages[m]; msg.Text != "" {
				ents = append(ents, rdEntry{note: true, text: msg.Text, msg: m, at: msg.At})
			}
			m++
		}
	} else {
		if a.Dropped > 0 {
			out = append(out, readItem{kind: readDropped, msg: -1, text: "… " + strconv.Itoa(a.Dropped) + " earlier steps"})
		}
		// Hook steps as transcript steps, so one path folds and words both.
		steps := make([]subagents.TranscriptStep, len(a.Steps))
		for i, h := range a.Steps {
			st := subagents.TranscriptStep{At: h.At, Tool: h.Tool, ID: "h" + strconv.Itoa(a.Dropped+i)}
			switch foldOf(h.Tool) {
			case foldReads, foldEdits:
				st.Path = h.Summary
			case foldSearches:
				st.Pattern = h.Summary
			default:
				st.Command, st.Query = h.Summary, h.Summary
			}
			steps[i] = st
		}
		var notes []rdEntry
		if tr != nil && tr.Available {
			if tr.Dropped {
				out = append(out, readItem{kind: readDropped, msg: -1, text: "… earlier messages hidden"})
			}
			var last time.Time
			for i, m := range tr.Messages {
				at := m.At
				if at.IsZero() {
					at = last
				}
				last = at
				if m.Text != "" {
					notes = append(notes, rdEntry{note: true, text: m.Text, msg: i, at: at})
				}
			}
		}
		ents = make([]rdEntry, 0, len(steps)+len(notes))
		s := 0
		for s < len(steps) || len(notes) > 0 {
			if len(notes) == 0 || (s < len(steps) && steps[s].At.Before(notes[0].at)) {
				ents = append(ents, rdEntry{step: &steps[s], at: steps[s].At})
				s++
				continue
			}
			ents = append(ents, notes[0])
			notes = notes[1:]
		}
	}
	var run []*subagents.TranscriptStep
	cls := foldNone
	flush := func() {
		if len(run) > 0 {
			out = append(out, foldItem(cls, run, hook))
			run = run[:0]
		}
	}
	for _, e := range ents {
		if e.note {
			flush()
			out = append(out, readItem{kind: readNote, at: e.at, text: e.text, msg: e.msg})
			continue
		}
		if plumbing(e.step.Tool) {
			continue
		}
		c := foldOf(e.step.Tool)
		if c == foldNone {
			flush()
			out = append(out, stepItem(e.step, hook))
			continue
		}
		if c != cls {
			flush()
			cls = c
		}
		run = append(run, e.step)
	}
	flush()
	return append(out, endItem(a))
}

// endItem is the waiting, finished or working row, as in dockTimeline.
func endItem(a *subagents.Agent) readItem {
	switch a.State {
	case subagents.Waiting:
		text := "Waiting for your approval in the session above."
		if n := len(a.Steps); n > 0 && a.Steps[n-1].Tool == "Bash" && a.Steps[n-1].Summary != "" {
			text = "Wants to run " + a.Steps[n-1].Summary + ". Approve it in the session above."
		}
		return readItem{kind: readWaiting, text: text, msg: -1}
	case subagents.Done:
		return readItem{kind: readFinished, text: "Finished in " + minutes(a.Ended.Sub(a.Started)), msg: -1}
	}
	return readItem{kind: readLive, text: "working", msg: -1}
}

// stepItem is one step that never folds: Bash, a commit, web and other tools.
func stepItem(st *subagents.TranscriptStep, hook bool) readItem {
	it := readItem{kind: readStep, id: st.ID, at: st.At, msg: -1}
	switch st.Tool {
	case "Bash", "exec_command", "shell":
		if st.Commit != "" {
			it.verb, it.obj = "Committed", `"`+st.Subject+`"`
			it.addMeta(st.Commit, toneOK)
			return it
		}
		it.verb, it.obj = bashSentence(st.Desc)
		if it.verb == "" {
			first, _, _ := strings.Cut(st.Command, "\n")
			it.verb, it.obj, it.objMono = "Ran", cutRunes(strings.TrimSpace(first), readCmdRunes), true
		}
		if hook {
			return it
		}
		switch {
		case st.Fails > 0:
			s := "failed · " + strconv.Itoa(st.Fails) + " test"
			if st.Fails > 1 {
				s += "s"
			}
			it.addMeta(s, toneBad)
		case st.Failed:
			it.addMeta("failed", toneBad)
		case st.Background:
			it.addMeta("started", tonePlain)
		case st.Done && testCommand(st.Command):
			it.addMeta("passed", toneOK)
		case st.Done:
			it.addMeta("done", toneOK)
		}
		if st.Command != "" {
			it.detail.cap = "$ " + cutRunes(strings.ReplaceAll(st.Command, "\n", " "), readCapRunes)
		}
		for _, o := range st.Out {
			it.detail.lines = append(it.detail.lines, subagents.DiffLine{Kind: ' ', Text: o})
		}
	case "WebFetch":
		it.verb, it.obj = "Fetched", st.Query
		if u, err := url.Parse(st.Query); err == nil && u.Host != "" {
			it.obj = u.Host
		}
	case "WebSearch":
		it.verb, it.obj = "Searched the web for", st.Query
	default:
		it.verb, it.obj, it.objMono = "Used", st.Tool, true
	}
	return it
}

// bashSentence turns Claude's description of a command into a past-tense
// sentence: the verb and the rest. ("", "") when there is no description; the
// whole description as the verb when its first word is not in the table.
func bashSentence(desc string) (verb, rest string) {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return "", ""
	}
	first, rest, _ := strings.Cut(desc, " ")
	if past, ok := pastVerbs[strings.ToLower(first)]; ok {
		return past, rest
	}
	r, n := utf8.DecodeRuneInString(desc)
	return string(unicode.ToUpper(r)) + desc[n:], ""
}

// foldItem is a run of one fold class: one step, or the folded sentence.
func foldItem(cls foldClass, run []*subagents.TranscriptStep, hook bool) readItem {
	it := readItem{kind: readStep, id: run[0].ID, at: run[0].At, msg: -1}
	switch cls {
	case foldReads:
		paths := distinctPaths(run, func(s *subagents.TranscriptStep) string { return s.Path })
		if len(paths) <= 1 {
			it.verb, it.obj, it.objMono = "Read", run[0].Path, true
			return it
		}
		it.verb, it.obj = "Read", strconv.Itoa(len(paths))+" files"
		it.detail.lines = listLines(paths)
	case foldSearches:
		it.verb = "Searched the code"
		if len(run) > 1 {
			it.obj = strconv.Itoa(len(run)) + " times"
		}
		if hook && len(run) == 1 {
			return it
		}
		for _, s := range run {
			if len(it.detail.lines) == readDetailLines {
				break
			}
			ln := s.Pattern
			if s.Path != "" {
				ln += " in " + s.Path
			}
			if ln != "" {
				it.detail.lines = append(it.detail.lines, subagents.DiffLine{Kind: 'l', Text: ln})
			}
		}
	case foldEdits:
		paths := distinctPaths(run, func(s *subagents.TranscriptStep) string { return s.Path })
		added, removed := 0, 0
		for _, s := range run {
			added += s.Added
			removed += s.Removed
		}
		if len(paths) <= 1 {
			it.verb, it.obj, it.objMono = "Edited", run[0].Path, true
			for _, s := range run {
				for _, d := range s.Diff {
					if len(it.detail.lines) < readDetailLines {
						it.detail.lines = append(it.detail.lines, d)
					}
				}
			}
		} else {
			it.verb, it.obj = "Edited", strconv.Itoa(len(paths))+" files"
			for _, p := range paths {
				if len(it.detail.lines) == readDetailLines {
					break
				}
				ln := p
				if !hook {
					a, r := 0, 0
					for _, s := range run {
						if s.Path == p {
							a += s.Added
							r += s.Removed
						}
					}
					ln += "  +" + strconv.Itoa(a) + " −" + strconv.Itoa(r)
				}
				it.detail.lines = append(it.detail.lines, subagents.DiffLine{Kind: 'l', Text: ln})
			}
		}
		if !hook {
			it.addMeta("+"+strconv.Itoa(added), toneAdd)
			it.addMeta("−"+strconv.Itoa(removed), toneDel)
		}
	case foldTodos:
		it.verb = "Updated the to-do list"
	}
	return it
}

// distinctPaths are the run's distinct non-empty keys, first seen first.
func distinctPaths(run []*subagents.TranscriptStep, key func(*subagents.TranscriptStep) string) []string {
	var out []string
outer:
	for _, s := range run {
		k := key(s)
		if k == "" {
			continue
		}
		for _, o := range out {
			if o == k {
				continue outer
			}
		}
		out = append(out, k)
	}
	return out
}

func listLines(ss []string) []subagents.DiffLine {
	out := make([]subagents.DiffLine, 0, min(len(ss), readDetailLines))
	for _, s := range ss {
		if len(out) == readDetailLines {
			break
		}
		out = append(out, subagents.DiffLine{Kind: 'l', Text: s})
	}
	return out
}

// readableCounts is the header's "21 steps · 3 messages": every step,
// plumbing included, and the agent's text messages.
func readableCounts(a *subagents.Agent, tr *subagents.Transcript) (steps, messages int) {
	if tr != nil && tr.Available {
		for _, m := range tr.Messages {
			if m.Text != "" {
				messages++
			}
		}
		if len(tr.Steps) > 0 {
			return len(tr.Steps), messages
		}
	}
	return a.Dropped + len(a.Steps), messages
}
