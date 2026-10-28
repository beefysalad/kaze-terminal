package sound

import (
	"os"
	"os/exec"
	"sync"
)

var (
	tmpOnce sync.Once
	tmpPath string
)

// afplay needs a file, so write the bytes once and reuse the path.
func platformPlay(b []byte) {
	tmpOnce.Do(func() {
		f, err := os.CreateTemp("", "kaze-chime-*.wav")
		if err != nil {
			return
		}
		defer f.Close()
		if _, err := f.Write(b); err == nil {
			tmpPath = f.Name()
		}
	})
	if tmpPath == "" {
		return
	}
	cmd := exec.Command("afplay", tmpPath)
	if cmd.Start() == nil {
		go cmd.Wait() // reap the child; never block the caller
	}
}
