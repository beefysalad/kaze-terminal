package subagents

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/beefysalad/kaze-terminal/internal/notify"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MaxMessages          = 128
	MaxTranscriptBytes   = 128 << 10
	MaxTranscriptLine    = 256 << 10
	transcriptReadBudget = 256 << 10

	// Step caps: a huge edit or output can't grow memory past these.
	MaxTranscriptSteps = 200
	maxDiffLines       = 24
	maxOutLines        = 8
	maxDetailLine      = 200       // runes per kept diff or output line
	maxStepDetail      = 4 << 10   // detail bytes per step
	maxStepsDetail     = 256 << 10 // detail bytes for all steps; the oldest lose theirs first
	maxStepText        = 1 << 10   // bytes of a step's command
	resultLookBack     = 64        // steps scanned back for a tool_result's call
)

// Transcript keeps a bounded tail. Offset counts bytes consumed, including a
// partial record; pending holds only that record until its newline arrives.
// The shown dock tab owns this value and calls Read only while visible.
type Transcript struct {
	Messages           []TranscriptMessage
	Steps              []TranscriptStep // tool calls in record order, at most MaxTranscriptSteps
	Offset             int64
	Bytes              int
	Dropped            bool // anything was left out: messages, steps or an oversized line
	StepsDropped       bool // the oldest steps were trimmed past MaxTranscriptSteps
	Version            uint64
	Available          bool
	pending            []byte
	skipping           bool
	info               os.FileInfo
	handback, lastText string
	seq                uint64 // record order shared by Messages and Steps
	cwd                string
	stepBytes          int // detail bytes held by Steps
	detailFrom         int // Steps before this index hold no detail
}
type TranscriptMessage struct {
	Text  string
	Tools ToolCounts
	At    time.Time // when the record was written; zero when it had no usable timestamp
	Seq   uint64    // order in the file, shared with Steps
}

// DiffLine is one kept line of an edit: Kind ' ' (context), '+' or '-'.
type DiffLine struct {
	Kind byte
	Text string
}

// TranscriptStep is one tool call and, once its result arrives, its outcome.
// Diff and Out are capped detail; the other fields are the sentence facts.
type TranscriptStep struct {
	Seq                  uint64
	At                   time.Time
	ID, Tool             string
	Desc, Command        string // Bash: Claude's description and the command
	Path, Pattern, Query string // file or search path, Grep/Glob pattern, web query or URL
	Added, Removed       int    // edit line counts over the whole text
	Diff                 []DiffLine
	Out                  []string // the last lines of the result
	Done, Failed         bool
	Fails                int    // Go "--- FAIL:" lines in the result
	Commit, Subject      string // a git commit's short hash and subject
	Background           bool
}
type ToolCounts struct{ Searches, Reads, Directories, Commands, Edits, Web, Other int }

func (c *ToolCounts) add(name string) {
	switch name {
	case "Grep", "Glob":
		c.Searches++
	case "Read":
		c.Reads++
	case "LS", "ListDirectory":
		c.Directories++
	case "Bash":
		c.Commands++
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		c.Edits++
	case "WebSearch", "WebFetch":
		c.Web++
	default:
		c.Other++
	}
}
func (c ToolCounts) String() string {
	var parts []string
	add := func(n int, verb, one, many string) {
		if n == 0 {
			return
		}
		if n == 1 {
			parts = append(parts, fmt.Sprintf("%s 1 %s", verb, one))
		} else {
			parts = append(parts, fmt.Sprintf("%s %d %s", verb, n, many))
		}
	}
	add(c.Searches, "searched for", "pattern", "patterns")
	add(c.Reads, "read", "file", "files")
	add(c.Directories, "listed", "directory", "directories")
	add(c.Commands, "ran", "shell command", "shell commands")
	add(c.Edits, "edited", "file", "files")
	add(c.Web, "made", "web request", "web requests")
	add(c.Other, "used", "other tool", "other tools")
	s := strings.Join(parts, ", ")
	if s != "" {
		s = strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}
func TranscriptPath(main, id string) string {
	if !strings.HasSuffix(main, ".jsonl") || !filepath.IsAbs(main) || id == "" || len(id) > 256 {
		return ""
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return ""
		}
	}
	return filepath.Join(strings.TrimSuffix(main, ".jsonl"), "subagents", "agent-"+id+".jsonl")
}

// openFile opens a file the readers have checked; tests count the opens.
var openFile = os.Open

var errNotRegular = errors.New("not a regular file")

// openRegular opens path only when it is a regular file: a FIFO or a device
// is turned away before the open, which could block on it.
func openRegular(path string) (*os.File, os.FileInfo, error) {
	if st, err := os.Stat(path); err != nil {
		return nil, nil, err
	} else if !st.Mode().IsRegular() {
		return nil, nil, errNotRegular
	}
	f, err := openFile(path)
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		f.Close()
		return nil, nil, errNotRegular
	}
	return f, st, nil
}

func (tr *Transcript) Read(path string) error {
	f, st, e := openRegular(path)
	if e != nil {
		tr.Available = false
		return e
	}
	defer f.Close()
	if tr.info != nil && (!os.SameFile(tr.info, st) || st.Size() < tr.Offset) {
		v := tr.Version
		*tr = Transcript{Version: v + 1}
	}
	tr.info = st
	tr.Available = true
	if _, e = f.Seek(tr.Offset, io.SeekStart); e != nil {
		return e
	}
	var buf [32 << 10]byte
	for budget := transcriptReadBudget; budget > 0; {
		n, e := f.Read(buf[:min(len(buf), budget)])
		if n > 0 {
			tr.Offset += int64(n)
			budget -= n
			tr.feed(buf[:n])
		}
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		if n == 0 {
			break
		}
	}
	return nil
}
func (tr *Transcript) feed(data []byte) {
	for _, b := range data {
		if b == '\n' {
			if !tr.skipping && len(tr.pending) > 0 {
				tr.record(tr.pending)
			}
			tr.pending = tr.pending[:0]
			tr.skipping = false
			continue
		}
		if tr.skipping {
			continue
		}
		if len(tr.pending) >= MaxTranscriptLine {
			tr.pending = tr.pending[:0]
			tr.skipping = true
			tr.Dropped = true
			tr.Version++
			continue
		}
		tr.pending = append(tr.pending, b)
	}
}

// stepInput holds the tool_use input fields a step keeps.
type stepInput struct {
	Message     string `json:"message"`
	Description string `json:"description"`
	Command     string `json:"command"`
	FilePath    string `json:"file_path"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	Content     string `json:"content"`
	Edits       []struct {
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	} `json:"edits"`
	Pattern         string `json:"pattern"`
	Path            string `json:"path"`
	URL             string `json:"url"`
	Query           string `json:"query"`
	RunInBackground bool   `json:"run_in_background"`
}

func (tr *Transcript) record(line []byte) {
	var entry struct {
		Type      string `json:"type"`
		Timestamp string `json:"timestamp"`
		Cwd       string `json:"cwd"`
		Message   struct {
			Content []struct {
				Type, Text, Name, ID string
				ToolUseID            string          `json:"tool_use_id"`
				IsError              bool            `json:"is_error"`
				Content              json.RawMessage `json:"content"`
				Input                json.RawMessage `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	// A user record whose content is a plain string (a prompt) fails to
	// decode here and is skipped, as before.
	if json.Unmarshal(line, &entry) != nil {
		return
	}
	if entry.Cwd != "" && entry.Cwd != tr.cwd {
		tr.cwd = entry.Cwd
	}
	if entry.Type == "user" {
		for i := range entry.Message.Content {
			if b := &entry.Message.Content[i]; b.Type == "tool_result" && b.ToolUseID != "" {
				tr.result(b.ToolUseID, b.IsError, b.Content)
			}
		}
		return
	}
	if entry.Type != "assistant" {
		return
	}
	at, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
	for i := range entry.Message.Content {
		b := &entry.Message.Content[i]
		switch b.Type {
		case "text":
			s := notify.CleanReport(b.Text, notify.MaxReport)
			if strings.TrimSpace(s) == "" {
				continue
			}
			tr.lastText = s
			tr.append(TranscriptMessage{Text: s, At: at})
		case "tool_use":
			// Decoded per block, errors ignored: a tool whose input has an
			// unexpected shape keeps the fields that did decode and never
			// costs the record its text.
			var in stepInput
			json.Unmarshal(b.Input, &in)
			if b.Name == "SubagentHandback" {
				s := notify.CleanReport(in.Message, notify.MaxReport)
				if strings.TrimSpace(s) != "" {
					tr.handback = s
					tr.Version++
				}
				continue
			}
			if len(tr.Messages) > 0 && tr.Messages[len(tr.Messages)-1].Text == "" {
				tr.Messages[len(tr.Messages)-1].Tools.add(b.Name)
				tr.Version++
			} else {
				m := TranscriptMessage{At: at}
				m.Tools.add(b.Name)
				tr.append(m)
			}
			tr.addStep(tr.newStep(b.ID, b.Name, at, &in))
		}
	}
}

// newStep keeps the facts a sentence needs and the capped detail.
func (tr *Transcript) newStep(id, name string, at time.Time, in *stepInput) TranscriptStep {
	st := TranscriptStep{At: at, ID: cutStepBytes(id, 128), Tool: notify.CleanLine(name, 64)}
	switch name {
	case "Bash":
		st.Desc = stepLine(in.Description)
		st.Command = cutStepBytes(notify.CleanReport(strings.TrimSpace(in.Command), maxStepText), maxStepText)
		st.Background = in.RunInBackground
	case "Read", "NotebookEdit":
		st.Path = shortPath(in.FilePath, tr.cwd)
	case "Edit":
		st.Path = shortPath(in.FilePath, tr.cwd)
		st.Added, st.Removed, st.Diff = lineDiff(in.OldString, in.NewString, nil)
	case "MultiEdit":
		st.Path = shortPath(in.FilePath, tr.cwd)
		for _, e := range in.Edits {
			a, r, d := lineDiff(e.OldString, e.NewString, st.Diff)
			st.Added += a
			st.Removed += r
			st.Diff = d
		}
	case "Write":
		st.Path = shortPath(in.FilePath, tr.cwd)
		st.Added = countLines(in.Content)
		st.Diff = writeDiff(in.Content)
	case "Grep", "Glob":
		st.Pattern = stepLine(in.Pattern)
		if in.Path != "" {
			st.Path = shortPath(in.Path, tr.cwd)
		}
	case "WebFetch":
		st.Query = stepLine(in.URL)
	case "WebSearch":
		st.Query = stepLine(in.Query)
	}
	return st
}

func (tr *Transcript) addStep(st TranscriptStep) {
	st.Seq = tr.seq
	tr.seq++
	tr.Steps = append(tr.Steps, st)
	tr.stepBytes += stepDetail(&st)
	tr.Version++
	if len(tr.Steps) > MaxTranscriptSteps {
		tr.stepBytes -= stepDetail(&tr.Steps[0])
		copy(tr.Steps, tr.Steps[1:])
		tr.Steps[len(tr.Steps)-1] = TranscriptStep{}
		tr.Steps = tr.Steps[:len(tr.Steps)-1]
		tr.detailFrom = max(0, tr.detailFrom-1)
		tr.Dropped, tr.StepsDropped = true, true
	}
	tr.trimDetail()
}

// trimDetail takes the detail of the oldest steps until all of it fits
// maxStepsDetail. The newest step always keeps its own.
func (tr *Transcript) trimDetail() {
	for tr.stepBytes > maxStepsDetail && tr.detailFrom < len(tr.Steps)-1 {
		s := &tr.Steps[tr.detailFrom]
		tr.stepBytes -= stepDetail(s)
		s.Diff, s.Out = nil, nil
		tr.detailFrom++
	}
}

// result fills in a call's outcome. The call is looked for among the last
// resultLookBack steps (no map); an unknown id is ignored.
func (tr *Transcript) result(id string, isErr bool, raw json.RawMessage) {
	at := -1
	for i := len(tr.Steps) - 1; i >= 0 && i >= len(tr.Steps)-resultLookBack; i-- {
		if tr.Steps[i].ID == id {
			at = i
			break
		}
	}
	if at < 0 || tr.Steps[at].Done {
		return
	}
	st := &tr.Steps[at]
	text := resultText(raw)
	st.Done, st.Failed = true, isErr
	for rest := text; rest != ""; {
		var ln string
		ln, rest, _ = strings.Cut(rest, "\n")
		if strings.HasPrefix(ln, "--- FAIL:") {
			st.Fails++
		}
	}
	if st.Tool == "Bash" && strings.Contains(st.Command, "git commit") {
		st.Commit, st.Subject = commitLine(text)
	}
	switch st.Tool {
	case "Bash", "exec_command", "shell": // the only steps that show their output
		if at >= tr.detailFrom {
			before := stepDetail(st)
			st.Out = tailLines(text, maxStepDetail-before)
			tr.stepBytes += stepDetail(st) - before
		}
	}
	tr.Version++
	tr.trimDetail()
}

// resultText is a tool_result's content: a JSON string, or the text blocks of
// an array joined by newlines. Other blocks (images) are skipped.
func resultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		json.Unmarshal(raw, &s)
		return s
	case '[':
		var blocks []struct{ Type, Text string }
		if json.Unmarshal(raw, &blocks) != nil {
			return ""
		}
		var b strings.Builder
		for _, x := range blocks {
			if x.Type != "text" {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(x.Text)
		}
		return b.String()
	}
	return ""
}

// tailLines keeps the last maxOutLines non-empty lines of s, each cut to
// maxDetailLine runes, within budget bytes, oldest first.
func tailLines(s string, budget int) []string {
	var keep [maxOutLines]string
	n, used := 0, 0
	for end := len(s); end > 0 && n < maxOutLines; {
		start := strings.LastIndexByte(s[:end], '\n') + 1
		ln := s[start:end]
		end = max(0, start-1)
		if strings.TrimSpace(ln) == "" {
			continue
		}
		d := detailLine(ln)
		if used+len(d) > budget {
			break
		}
		used += len(d)
		keep[maxOutLines-1-n] = d
		n++
	}
	if n == 0 {
		return nil
	}
	return append([]string(nil), keep[maxOutLines-n:]...)
}

// commitLine reads "[<branch> <hash>] <subject>" from git commit's output.
func commitLine(s string) (hash, subject string) {
	for rest := s; rest != ""; {
		var ln string
		ln, rest, _ = strings.Cut(rest, "\n")
		ln = strings.TrimSpace(ln)
		inner, subj, ok := strings.Cut(ln, "] ")
		if !ok || !strings.HasPrefix(inner, "[") {
			continue
		}
		h := inner[strings.LastIndexByte(inner, ' ')+1:]
		if len(h) < 7 || len(h) > 40 || strings.TrimLeft(h, "0123456789abcdef") != "" {
			continue
		}
		return h, stepLine(subj)
	}
	return "", ""
}

// lineDiff counts the lines an edit adds and removes (common leading and
// trailing lines trimmed) and appends to diff, within the caps, up to two
// context lines on each side, the removed lines, then the added ones.
func lineDiff(old, new string, diff []DiffLine) (added, removed int, out []DiffLine) {
	a, b := splitLines(old), splitLines(new)
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	removed, added = len(a)-p-s, len(b)-p-s
	used := 0
	for _, d := range diff {
		used += len(d.Text)
	}
	put := func(kind byte, lines []string) {
		for _, l := range lines {
			if len(diff) >= maxDiffLines {
				return
			}
			d := detailLine(l)
			if used+len(d) > maxStepDetail {
				return
			}
			used += len(d)
			diff = append(diff, DiffLine{kind, d})
		}
	}
	put(' ', a[max(0, p-2):p])
	put('-', a[p:len(a)-s])
	put('+', b[p:len(b)-s])
	put(' ', a[len(a)-s:min(len(a), len(a)-s+2)])
	return added, removed, diff
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func countLines(s string) int {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// writeDiff is a new file's first lines as additions, within the caps.
func writeDiff(s string) []DiffLine {
	var diff []DiffLine
	used := 0
	for rest := strings.TrimSuffix(s, "\n"); rest != "" && len(diff) < maxDiffLines; {
		var ln string
		ln, rest, _ = strings.Cut(rest, "\n")
		d := detailLine(ln)
		if used+len(d) > maxStepDetail {
			break
		}
		used += len(d)
		diff = append(diff, DiffLine{'+', d})
	}
	return diff
}

func stepDetail(s *TranscriptStep) int {
	n := 0
	for _, d := range s.Diff {
		n += len(d.Text)
	}
	for _, o := range s.Out {
		n += len(o)
	}
	return n
}

// detailLine is one line fit to show: tabs as four spaces, control runes
// dropped, at most maxDetailLine runes.
func detailLine(s string) string {
	if strings.IndexByte(s, '\t') >= 0 {
		s = strings.ReplaceAll(s, "\t", "    ")
	}
	return notify.CleanLine(s, maxDetailLine)
}

// stepLine is the first line of s, cleaned and cut to maxDetailLine runes.
func stepLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(notify.CleanLine(s, maxDetailLine))
}

// shortPath shows p relative to cwd when inside it, else its last 3 parts,
// always with forward slashes.
func shortPath(p, cwd string) string {
	p = notify.CleanLine(p, 1024)
	if cwd != "" && len(p) > len(cwd)+1 && strings.EqualFold(p[:len(cwd)], cwd) && (p[len(cwd)] == '/' || p[len(cwd)] == '\\') {
		return strings.ReplaceAll(p[len(cwd)+1:], "\\", "/")
	}
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) > 3 {
		parts = parts[len(parts)-3:]
	}
	return strings.Join(parts, "/")
}

// cutStepBytes cuts s to max bytes on a rune boundary.
func cutStepBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && s[max]&0xC0 == 0x80 {
		max--
	}
	return s[:max]
}

func (tr *Transcript) append(m TranscriptMessage) {
	m.Seq = tr.seq
	tr.seq++
	tr.Messages = append(tr.Messages, m)
	tr.Bytes += len(m.Text)
	tr.Version++
	for len(tr.Messages) > MaxMessages || tr.Bytes > MaxTranscriptBytes {
		tr.Bytes -= len(tr.Messages[0].Text)
		copy(tr.Messages, tr.Messages[1:])
		tr.Messages[len(tr.Messages)-1] = TranscriptMessage{}
		tr.Messages = tr.Messages[:len(tr.Messages)-1]
		tr.Dropped = true
	}
}
func (tr *Transcript) Report(hook string) string {
	if s := notify.CleanReport(hook, notify.MaxReport); strings.TrimSpace(s) != "" {
		return s
	}
	if tr.handback != "" {
		return tr.handback
	}
	if tr.lastText != "" {
		return tr.lastText
	}
	return "No report"
}

// AtEnd reports whether the last Read reached the end of the file it saw.
func (tr *Transcript) AtEnd() bool { return tr.info != nil && tr.Offset >= tr.info.Size() }
