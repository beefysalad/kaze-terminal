// Package sound plays kaze's alert chime without ever blocking the UI.
package sound

import (
	"os"
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

// New returns a Player for file; empty, or unreadable, means the bundled chime.
func New(file string) *Player { return newWith(file, platformPlay) }

func newWith(file string, play func([]byte)) *Player {
	data := sounds.Chime
	if file != "" {
		if b, err := os.ReadFile(file); err == nil && len(b) > 0 {
			data = b
		}
	}
	return &Player{data: data, play: play}
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
