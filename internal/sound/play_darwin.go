package sound

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var (
	pathMu sync.Mutex
	paths  = map[string]string{} // cache path -> confirmed on disk
)

// afplay needs a file. Each sound gets its own fixed per-user file, written
// once, so a second Player with other bytes never plays the first one's sound.
func platformPlay(b []byte) {
	path := ensureFile(b)
	if path == "" {
		return
	}
	cmd := exec.Command("afplay", path)
	if cmd.Start() == nil {
		go cmd.Wait() // reap the child; never block the caller
	}
}

func ensureFile(b []byte) string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	path := cachePath(dir, b)
	pathMu.Lock()
	defer pathMu.Unlock()
	if _, ok := paths[path]; ok {
		return path
	}
	if st, err := os.Stat(path); err == nil && st.Size() == int64(len(b)) {
		paths[path] = path
		return path
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path, b, 0o644) != nil {
		return ""
	}
	paths[path] = path
	return path
}
