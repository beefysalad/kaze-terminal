package tour

import (
	"os"
	"path/filepath"
	"runtime"
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
func TestMarkSeenWriteFailure(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "file")
	os.WriteFile(p, []byte("x"), 0600)
	if MarkSeen(p) == nil {
		t.Fatal("expected failure")
	}
}
func TestMarkSeenReadOnlyDirReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions")
	}
	d := t.TempDir()
	if e := os.Chmod(d, 0500); e != nil {
		t.Fatal(e)
	}
	defer os.Chmod(d, 0700)
	if e := MarkSeen(d); e == nil {
		t.Fatal("expected permission error")
	}
}
