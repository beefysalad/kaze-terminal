package discord

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Dial tries the local desktop client's sockets in the first configured directory.
func Dial() (io.ReadWriteCloser, error) {
	dir := "/tmp"
	for _, key := range []string{"XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
		if value := os.Getenv(key); value != "" {
			dir = value
			break
		}
	}
	var err error
	for i := 0; i < 10; i++ {
		var c net.Conn
		c, err = net.DialTimeout("unix", filepath.Join(dir, fmt.Sprintf("discord-ipc-%d", i)), time.Second)
		if err == nil {
			return c, nil
		}
	}
	return nil, err
}
