package usage

import (
	"strings"
	"unicode"
	"unicode/utf8"
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
	if len(name) > maxModelName {
		cut := maxModelName
		for cut > 0 && !utf8.RuneStart(name[cut]) {
			cut--
		}
		name = name[:cut]
	}
	return name
}

func claudeModelName(id string) string {
	i := strings.Index(id, "claude-")
	if i < 0 {
		return ""
	}
	id = id[i+len("claude-"):]
	// "[1m]", a Vertex "@date" and a Bedrock ":0" all follow the name.
	if end := strings.IndexAny(id, "[@:"); end >= 0 {
		id = id[:end]
	}
	var words, nums []string
	for _, p := range strings.Split(id, "-") {
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
	return words[0] + " " + strings.Join(nums, ".") // later words are "latest", "thinking" and the like
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
