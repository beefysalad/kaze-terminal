// Package codecolor colours a single line of source code. It is a small
// hand-written scanner, not a parser: it knows keywords, strings, comments,
// numbers and names for a handful of common languages, and it looks at one
// line at a time, so a string or comment that spans lines is not carried over.
package codecolor

import (
	"path/filepath"
	"sort"
	"strings"
)

// Lang names a language the scanner knows.
type Lang uint8

// The languages. None means "do not colour".
const (
	None Lang = iota
	Go
	Python
	TypeScript
	JavaScript
	Shell
	JSON
)

// Kind is the class of a coloured span.
type Kind uint8

// The span kinds. Plain text gets no span.
const (
	Plain Kind = iota
	Keyword
	String
	Comment
	Number
	Name
)

// Span is a coloured byte range [Start, End) of a line.
type Span struct {
	Start, End int
	Kind       Kind
}

type table struct {
	keywords []string // sorted
	comment  string   // line comment marker
	quotes   string
	capName  bool // a capitalised word is a Name
	json     bool
}

var tables = [...]table{
	Go: {
		keywords: []string{"break", "case", "chan", "const", "continue", "default", "defer", "else", "false", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "nil", "package", "range", "return", "select", "struct", "switch", "true", "type", "var"},
		comment:  "//", quotes: "\"`'", capName: true,
	},
	Python: {
		keywords: []string{"False", "None", "True", "and", "as", "assert", "async", "await", "break", "class", "continue", "def", "del", "elif", "else", "except", "finally", "for", "from", "global", "if", "import", "in", "is", "lambda", "nonlocal", "not", "or", "pass", "raise", "return", "try", "while", "with", "yield"},
		comment:  "#", quotes: "\"'",
	},
	TypeScript: {
		keywords: tsKeywords,
		comment:  "//", quotes: "\"'`", capName: true,
	},
	JavaScript: {
		keywords: jsKeywords,
		comment:  "//", quotes: "\"'`", capName: true,
	},
	Shell: {
		keywords: []string{"case", "do", "done", "elif", "else", "esac", "fi", "for", "function", "if", "in", "select", "then", "until", "while"},
		comment:  "#", quotes: "\"'",
	},
	JSON: {
		keywords: []string{"false", "null", "true"},
		quotes:   "\"", json: true,
	},
}

var jsKeywords = []string{"async", "await", "break", "case", "catch", "class", "const", "continue", "default", "delete", "do", "else", "export", "extends", "false", "finally", "for", "from", "function", "if", "import", "in", "instanceof", "let", "new", "null", "of", "return", "super", "switch", "this", "throw", "true", "try", "typeof", "undefined", "var", "void", "while", "yield"}

var tsKeywords = []string{"abstract", "as", "async", "await", "break", "case", "catch", "class", "const", "continue", "default", "delete", "do", "else", "enum", "export", "extends", "false", "finally", "for", "from", "function", "if", "implements", "import", "in", "instanceof", "interface", "let", "namespace", "new", "null", "of", "private", "protected", "public", "readonly", "return", "super", "switch", "this", "throw", "true", "try", "type", "typeof", "undefined", "var", "void", "while", "yield"}

func init() {
	for i := range tables {
		sort.Strings(tables[i].keywords)
	}
}

// LangOf picks a language from a file path's extension, ignoring case.
func LangOf(path string) Lang {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return Go
	case ".py":
		return Python
	case ".ts", ".tsx":
		return TypeScript
	case ".js", ".jsx":
		return JavaScript
	case ".sh", ".bash":
		return Shell
	case ".json":
		return JSON
	}
	return None
}

func isWordByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func (t *table) isKeyword(w string) bool {
	i := sort.SearchStrings(t.keywords, w)
	return i < len(t.keywords) && t.keywords[i] == w
}

// Line appends the coloured spans of s to dst and returns it. Spans are in
// order, do not overlap and sit on rune boundaries; plain text has none. With
// None it returns dst unchanged. It does not allocate when dst has room.
func Line(l Lang, s string, dst []Span) []Span {
	if l == None || int(l) >= len(tables) {
		return dst
	}
	t := &tables[l]
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case t.comment != "" && strings.HasPrefix(s[i:], t.comment) && (t.comment != "#" || l != Shell || i == 0 || s[i-1] == ' ' || s[i-1] == '\t'):
			dst = append(dst, Span{i, len(s), Comment})
			return dst
		case strings.IndexByte(t.quotes, c) >= 0:
			j := i + 1
			for j < len(s) && s[j] != c {
				if s[j] == 0x5c { // backslash
					j++
				}
				j++
			}
			if j < len(s) {
				j++ // the closing quote
			} else {
				j = len(s)
			}
			k := String
			if t.json {
				m := j
				for m < len(s) && (s[m] == ' ' || s[m] == '\t') {
					m++
				}
				if m < len(s) && s[m] == ':' {
					k = Name
				}
			}
			dst = append(dst, Span{i, j, k})
			i = j
		case isDigit(c) || t.json && c == '-' && i+1 < len(s) && isDigit(s[i+1]):
			j := i + 1
			hex := c == '0' && j < len(s) && (s[j] == 'x' || s[j] == 'X')
			for j < len(s) {
				d := s[j]
				if isWordByte(d) && d < 0x80 || d == '.' {
					j++
				} else if (d == '+' || d == '-') && !hex && (s[j-1] == 'e' || s[j-1] == 'E') {
					j++
				} else {
					break
				}
			}
			dst = append(dst, Span{i, j, Number})
			i = j
		case isWordByte(c):
			j := i + 1
			for j < len(s) && isWordByte(s[j]) {
				j++
			}
			w := s[i:j]
			if t.isKeyword(w) {
				dst = append(dst, Span{i, j, Keyword})
			} else if j < len(s) && s[j] == '(' || t.capName && w[0] >= 'A' && w[0] <= 'Z' {
				dst = append(dst, Span{i, j, Name})
			}
			i = j
		default:
			i++
		}
	}
	return dst
}
