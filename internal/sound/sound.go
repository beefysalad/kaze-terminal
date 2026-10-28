// Package sound plays kaze's alert chime without ever blocking the UI.
package sound

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/patrick-gogo/kaze-terminal/assets/sounds"
)

// coalesce: several alerts close together should give one chime.
const coalesce = time.Second

type Player struct {
	data []byte
	play func([]byte)

	mu   sync.Mutex
	last time.Time
}

// maxFileBytes caps a custom chime so a wrong path can't load a huge file.
const maxFileBytes = 2 << 20

// New returns a Player for file; empty means the bundled chime. A file that
// is unreadable, empty, too big or not a WAV falls back to the bundled chime
// and the second result says why (empty when all is well).
func New(file string) (*Player, string) { return newWith(file, platformPlay) }

func newWith(file string, play func([]byte)) (*Player, string) {
	data, warn := sounds.Chime, ""
	if file != "" {
		if b, err := readWav(file); err != nil {
			warn = fmt.Sprintf("chime_file %q: %v; using the bundled chime", file, err)
		} else {
			data = b
		}
	}
	return &Player{data: data, play: play}, warn
}

func readWav(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	switch {
	case len(b) == 0:
		return nil, errors.New("file is empty")
	case len(b) > maxFileBytes:
		return nil, errors.New("file is larger than 2MB")
	case len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE":
		return nil, errors.New("not a WAV file")
	}
	return b, nil
}

// cachePath is where macOS keeps the file for these bytes: one fixed file per
// distinct sound, so nothing piles up across launches.
func cachePath(dir string, b []byte) string {
	sum := sha256.Sum256(b)
	return filepath.Join(dir, "kaze", "chime-"+hex.EncodeToString(sum[:6])+".wav")
}

// Play starts the chime in the background and returns at once. Calls within
// one second of the last chime that actually played are dropped.
func (p *Player) Play(now time.Time) {
	p.mu.Lock()
	if !p.last.IsZero() && now.Sub(p.last) < coalesce {
		p.mu.Unlock()
		return
	}
	p.last = now
	p.mu.Unlock()
	go p.play(p.data)
}
