package sound

import (
	"sync"
	"syscall"
	"unsafe"
)

const (
	sndAsync     = 0x0001
	sndNoDefault = 0x0002
	sndMemory    = 0x0004
)

var (
	playSoundW = syscall.NewLazyDLL("winmm.dll").NewProc("PlaySoundW")

	// PlaySound with SND_ASYNC keeps reading the buffer after it returns,
	// so hold the latest one alive.
	keepMu   sync.Mutex
	keepData []byte
)

func platformPlay(b []byte) {
	if len(b) == 0 {
		return
	}
	keepMu.Lock()
	keepData = b
	keepMu.Unlock()
	playSoundW.Call(uintptr(unsafe.Pointer(&b[0])), 0, sndMemory|sndAsync|sndNoDefault)
}
