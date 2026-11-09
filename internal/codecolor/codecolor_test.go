package codecolor

import (
	"testing"
	"unicode/utf8"
)

func TestLangOf(t *testing.T) {
	for _, p := range []string{"a.go", "a.py", "a.ts", "a.tsx", "a.js", "a.jsx", "a.sh", "a.bash", "a.json", "dir/A.GO", "x.Json"} {
		if LangOf(p) == None {
			t.Errorf("LangOf(%q) = None", p)
		}
	}
	for _, p := range []string{"a.rb", "README", ""} {
		if LangOf(p) != None {
			t.Errorf("LangOf(%q) != None", p)
		}
	}
}

// kindOf returns the kind of a span that exactly covers some occurrence of
// sub in s, or Plain when no span does.
func kindOf(spans []Span, s, sub string) Kind {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] != sub {
			continue
		}
		for _, sp := range spans {
			if sp.Start == i && sp.End == i+len(sub) {
				return sp.Kind
			}
		}
	}
	return Plain
}

func check(t *testing.T, l Lang, s string, want map[string]Kind) {
	t.Helper()
	spans := Line(l, s, nil)
	for sub, k := range want {
		if got := kindOf(spans, s, sub); got != k {
			t.Errorf("%q in %q: kind %d, want %d (spans %v)", sub, s, got, k, spans)
		}
	}
}

func TestLineGo(t *testing.T) {
	check(t, LangOf("a.go"), `func main() { return "x" // y`, map[string]Kind{
		"func": Keyword, "return": Keyword, "main": Name, `"x"`: String, "// y": Comment,
	})
	check(t, LangOf("a.go"), `x := Foo{1}`, map[string]Kind{"Foo": Name, "1": Number})
}

func TestLinePython(t *testing.T) {
	check(t, LangOf("a.py"), `async def f(a): # c`, map[string]Kind{
		"async": Keyword, "def": Keyword, "f": Name, "# c": Comment,
	})
}

func TestLineTS(t *testing.T) {
	check(t, LangOf("a.ts"), `const x = Foo("a", 42) // z`, map[string]Kind{
		"const": Keyword, "Foo": Name, `"a"`: String, "42": Number, "// z": Comment,
	})
	check(t, LangOf("a.js"), "let s = `t`", map[string]Kind{"let": Keyword, "`t`": String})
}

func TestLineShell(t *testing.T) {
	check(t, LangOf("a.sh"), `if [ -n "$X" ]; then echo 5 # n`, map[string]Kind{
		"if": Keyword, "then": Keyword, `"$X"`: String, "5": Number, "# n": Comment,
	})
}

func TestLineJSON(t *testing.T) {
	check(t, LangOf("a.json"), `{"a": "b", "n": -1.5e3, "ok": true}`, map[string]Kind{
		`"a"`: Name, `"b"`: String, "-1.5e3": Number, "true": Keyword,
	})
}

func TestLineSpansCoverInOrder(t *testing.T) {
	lines := []string{
		``, `   `, `func main() { return "x" // y`, `s := "日本語\"x" + 'a' // コメント`,
		`async def f(a): # 日本`, `"unterminated`, `x := 0x1F + 3.14`, `}}}(("`,
		"let s = `日本`; // é",
	}
	for _, l := range []Lang{LangOf("a.go"), LangOf("a.py"), LangOf("a.ts"), LangOf("a.sh"), LangOf("a.json")} {
		for _, s := range lines {
			spans := Line(l, s, nil)
			prev := 0
			for _, sp := range spans {
				if sp.Start < prev || sp.End <= sp.Start || sp.End > len(s) {
					t.Fatalf("lang %d %q: bad span %v after %d", l, s, sp, prev)
				}
				if !utf8.RuneStart(s[sp.Start]) || (sp.End < len(s) && !utf8.RuneStart(s[sp.End])) {
					t.Fatalf("lang %d %q: span %v off a rune boundary", l, s, sp)
				}
				prev = sp.End
			}
		}
	}
}

func TestLineUnterminatedString(t *testing.T) {
	s := `"abc`
	spans := Line(LangOf("a.go"), s, nil)
	if len(spans) != 1 || spans[0] != (Span{0, 4, String}) {
		t.Fatalf("spans = %v", spans)
	}
}

func TestLineNoneIsNil(t *testing.T) {
	dst := make([]Span, 1, 4)
	got := Line(None, "func x", dst)
	if len(got) != 1 || cap(got) != 4 {
		t.Fatalf("got %v", got)
	}
}

func TestLineDoesNotAllocateWithRoom(t *testing.T) {
	dst := make([]Span, 0, 64)
	l := LangOf("a.go")
	s := `func main() { return "x" // y`
	if n := testing.AllocsPerRun(100, func() { dst = Line(l, s, dst[:0]) }); n != 0 {
		t.Fatalf("allocs = %v", n)
	}
}
