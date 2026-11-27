package tour

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck(t *testing.T) {
	d := t.TempDir()
	if Check(d) != Show {
		t.Fatal("fresh")
	}
	for _, n := range []string{"config.toml", "sessions.json", "tour.json"} {
		p := filepath.Join(d, n)
		os.WriteFile(p, []byte("broken"), 0600)
		if Check(d) != Skip {
			t.Fatal(n)
		}
		os.Remove(p)
	}
	if Check("") != Skip {
		t.Fatal("empty")
	}
	es, _ := os.ReadDir(d)
	if len(es) != 0 {
		t.Fatal("created files")
	}
	if e := MarkSeen(d); e != nil {
		t.Fatal(e)
	}
	if Check(d) != Skip {
		t.Fatal("seen")
	}
}
