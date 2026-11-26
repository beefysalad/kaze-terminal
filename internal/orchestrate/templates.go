package orchestrate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxTemplateBytes = 32 << 10
const MaxTemplates = 64

var templateName = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
var modelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)

// ValidTemplateName excludes the built-ins, which always keep their meaning.
func ValidTemplateName(name string) bool {
	return templateName.MatchString(name) && name != "claude" && name != "codex"
}

type Template struct{ Name, Agent, Description, Model, Mode, Body string }
type TemplateCatalog struct {
	Folder    string
	Templates []Template
	Warnings  []string
}

// LoadTemplate reads fresh, through a confined directory handle. Symlink files
// are refused, including links inside the folder, and reads are size bounded.
func LoadTemplate(folder, name string) (Template, []string, error) {
	if !ValidTemplateName(name) {
		return Template{}, nil, failure(CodeUnknownTemplate, "invalid or reserved template name")
	}
	if folder == "" {
		return Template{}, nil, failure(CodeUnknownTemplate, "template folder unavailable")
	}
	root, err := os.OpenRoot(folder)
	if err != nil {
		return Template{}, nil, failure(CodeUnknownTemplate, "cannot open template folder")
	}
	defer root.Close()
	return loadTemplate(root, name)
}
func loadTemplate(root *os.Root, name string) (Template, []string, error) {
	bad := func(reason string) (Template, []string, error) {
		return Template{}, nil, failure(CodeUnknownTemplate, name+": "+reason)
	}
	st, err := root.Lstat(name + ".md")
	if err != nil {
		return bad("template unavailable")
	}
	if !st.Mode().IsRegular() {
		return bad("template must be a regular file without symlinks")
	}
	if st.Size() > MaxTemplateBytes {
		return bad("template exceeds 32 KiB")
	}
	f, err := openCheckedTemplate(root, name+".md", st)
	if err != nil {
		return bad("cannot read template")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxTemplateBytes+1))
	if err != nil || len(b) > MaxTemplateBytes {
		return bad("cannot read template within 32 KiB")
	}
	if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
		return bad("template must be UTF-8 without NUL")
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	front := strings.TrimPrefix(text, "---\n")
	header, body, ok := strings.Cut(front, "\n---\n")
	if !ok && strings.HasSuffix(front, "\n---") {
		header = strings.TrimSuffix(front, "\n---")
		body = ""
		ok = true
	}
	if !strings.HasPrefix(text, "---\n") || !ok {
		return bad("frontmatter with agent is required")
	}
	t := Template{Name: name, Body: body}
	var warnings []string
	seen := map[string]bool{}
	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		if !ok {
			return bad("frontmatter must contain key: value lines")
		}
		switch key {
		case "description", "agent", "model", "mode":
		default:
			if len(warnings) < 64 {
				warnings = append(warnings, name+": ignored unknown frontmatter key "+cleanTemplateText(key, 64))
			}
			continue
		}
		if seen[key] {
			return bad("duplicate frontmatter key " + key)
		}
		seen[key] = true
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, `"`) {
			v, e := strconv.Unquote(value)
			if e != nil {
				return bad("invalid quoted value")
			}
			value = v
		} else if strings.HasPrefix(value, "'") {
			if len(value) < 2 || !strings.HasSuffix(value, "'") {
				return bad("invalid quoted value")
			}
			value = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
		}
		if strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return bad("frontmatter values cannot contain controls")
		}
		switch key {
		case "description":
			if len(value) > 512 {
				return bad("description exceeds 512 bytes")
			}
			t.Description = value
		case "agent":
			t.Agent = value
		case "model":
			t.Model = value
		case "mode":
			t.Mode = value
		}
	}
	if t.Agent != "claude" && t.Agent != "codex" {
		return bad("agent must be claude or codex")
	}
	if t.Mode != "" && t.Mode != "edit" && t.Mode != "read-only" {
		return bad("mode must be edit or read-only")
	}
	if t.Model != "" && !modelName.MatchString(t.Model) {
		return bad("invalid model name")
	}
	return t, warnings, nil
}
func cleanTemplateText(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	if len(s) > n {
		s = s[:n]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s
}

// ListTemplates bounds directory scanning as well as output. Broken files are
// skipped with reasons. A missing folder is the normal empty state.
func ListTemplates(folder string) TemplateCatalog {
	c := TemplateCatalog{Folder: folder}
	if folder == "" {
		c.Warnings = []string{"template folder unavailable"}
		return c
	}
	root, err := os.OpenRoot(folder)
	if errors.Is(err, os.ErrNotExist) {
		return c
	}
	if err != nil {
		c.Warnings = []string{"cannot open template folder"}
		return c
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		c.Warnings = []string{"cannot list templates"}
		return c
	}
	defer dir.Close()
	entries, err := dir.ReadDir(1025)
	if err != nil && err != io.EOF {
		c.Warnings = append(c.Warnings, "cannot list templates")
		return c
	}
	if len(entries) > 1024 {
		entries = entries[:1024]
		c.Warnings = append(c.Warnings, "template directory scan limited to 1024 entries")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok {
			continue
		}
		if len(c.Templates) == MaxTemplates {
			c.Warnings = append(c.Warnings, "template list limited to 64 entries")
			break
		}
		if !ValidTemplateName(name) {
			if len(c.Warnings) < 128 {
				c.Warnings = append(c.Warnings, "invalid or reserved template filename: "+cleanTemplateText(e.Name(), 64))
			}
			continue
		}
		t, w, err := loadTemplate(root, name)
		if err != nil {
			w = []string{err.Error()}
		} else {
			c.Templates = append(c.Templates, t)
		}
		for _, s := range w {
			if len(c.Warnings) < 128 {
				c.Warnings = append(c.Warnings, s)
			}
		}
	}
	return c
}

// ResolveTemplate defaults mode from the file; either side's read-only wins.
func ResolveTemplate(folder string, r Request) (Request, []string, error) {
	if r.Agent == "claude" || r.Agent == "codex" {
		return r, nil, nil
	}
	t, w, err := LoadTemplate(folder, r.Agent)
	if err != nil {
		return Request{}, w, err
	}
	if r.Mode != "" && r.Mode != "edit" && r.Mode != "read-only" {
		return Request{}, w, failure(CodeInvalidRequest, "invalid mode")
	}
	r.Agent = t.Agent
	r.Template = t.Name
	r.Model = t.Model
	r.Instructions = t.Body
	if r.Mode == "" || t.Mode == "read-only" {
		r.Mode = t.Mode
	}
	return r, w, nil
}

// TemplateText is bounded plain text for the CLI. Bodies never leave the host.
func (c TemplateCatalog) TemplateText() string {
	if len(c.Templates) == 0 {
		return fmt.Sprintf("no templates (put <name>.md in %s)", c.Folder)
	}
	var lines []string
	for _, t := range c.Templates {
		lines = append(lines, t.Name+"  "+t.Agent+"  "+t.Description)
	}
	return strings.Join(lines, "\n")
}

// openCheckedTemplate verifies the opened handle against the preflight stat.
func openCheckedTemplate(root *os.Root, name string, checked os.FileInfo) (*os.File, error) {
	f, err := openTemplateHandle(root, name)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(checked, opened) {
		f.Close()
		return nil, fmt.Errorf("template changed while opening")
	}
	return f, nil
}
