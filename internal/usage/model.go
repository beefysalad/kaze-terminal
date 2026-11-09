package usage

import (
	"strings"
	"unicode"
)

const maxModelName = 40

// ModelName is the short name shown for a model. Claude ids are read directly,
// so the status bar and the subagent pane agree ("claude-opus-5-5" is
// "Opus 5.5"); anything else falls back to display, then to the id itself.
func ModelName(id, display string) string {
	name := claudeModelName(id)
	if name == "" {
		name = strings.TrimSpace(display)
	}
	if name == "" {
		name = strings.TrimSpace(id)
	}
	if r := []rune(name); len(r) > maxModelName {
		name = string(r[:maxModelName])
		for len(name) > maxModelName { // multi-byte runes
			r = r[:len(r)-1]
			name = string(r)
		}
	}
	return name
}

func claudeModelName(id string) string {
	id, _, _ = strings.Cut(id, "[") // "[1m]" and the like
	i := strings.Index(id, "claude-")
	if i < 0 {
		return ""
	}
	var words, nums []string
	for _, p := range strings.Split(id[i+len("claude-"):], "-") {
		switch {
		case p == "":
		case allOf(p, unicode.IsDigit):
			if len(p) <= 2 { // longer is a date stamp
				nums = append(nums, p)
			}
		case allOf(p, unicode.IsLetter):
			words = append(words, strings.ToUpper(p[:1])+p[1:])
		}
	}
	if len(words) == 0 || len(nums) == 0 {
		return ""
	}
	return strings.Join(words, " ") + " " + strings.Join(nums, ".")
}

func allOf(s string, f func(rune) bool) bool {
	for _, r := range s {
		if !f(r) {
			return false
		}
	}
	return true
}

// ModelFamily is the first word of a ModelName, for a narrow window.
func ModelFamily(name string) string {
	first, _, _ := strings.Cut(name, " ")
	return first
}
