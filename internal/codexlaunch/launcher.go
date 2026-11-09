package codexlaunch

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const markerEnv = "KAZE_CODEX_SHIM"
const tokenEnv = "KAZE_CODEX_SHIM_TOKEN"
const markerName = ".kaze-codex-launch.json"
const directoryPrefix = "kaze-codex-launch-"
const maxImage = 64 << 20

type manifest struct {
	Version       int
	Token, Source string
	PID           int
}
type Launcher struct {
	dir, token string
	zdot       string // private zsh startup folder, empty unless PrepareZsh ran
	once       sync.Once
	closeErr   error
}

func shimName(goos string) string {
	if goos == "windows" {
		return "codex.exe"
	}
	return "codex"
}

// New creates this Kaze process's private launcher. On Windows the copy is a
// console image; on macOS a hardlink avoids copying the running executable.
func New(executable string) (*Launcher, error) {
	var probe func(string, string) error
	if runtime.GOOS == "windows" {
		probe = probeShim // a fresh unsigned copy can be refused by App Control
	}
	return newLauncher(executable, probe)
}

func newLauncher(executable string, probe func(path, token string) error) (*Launcher, error) {
	source, err := filepath.Abs(executable)
	if err != nil {
		return nil, err
	}
	if st, e := os.Stat(source); e != nil || !st.Mode().IsRegular() {
		return nil, fmt.Errorf("kaze: Codex launcher source unavailable: %s", source)
	}
	dir, err := os.MkdirTemp("", directoryPrefix)
	if err != nil {
		return nil, err
	}
	l := &Launcher{dir: dir}
	good := false
	defer func() {
		if !good {
			_ = os.RemoveAll(dir)
		}
	}()
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		return nil, err
	}
	l.token = hex.EncodeToString(token[:])
	dest := filepath.Join(dir, shimName(runtime.GOOS))
	if err = materialize(source, dest, runtime.GOOS, os.Link); err != nil {
		return nil, err
	}
	data, _ := json.Marshal(manifest{Version: 1, Token: l.token, Source: source, PID: os.Getpid()})
	if err = os.WriteFile(filepath.Join(dir, markerName), data, 0o600); err != nil {
		return nil, err
	}
	if probe != nil {
		if err = probe(dest, l.token); err != nil {
			return nil, err
		}
	}
	good = true
	return l, nil
}

func materialize(source, dest, goos string, link func(string, string) error) error {
	if goos == "windows" {
		b, e := readBounded(source, maxImage)
		if e != nil {
			return e
		}
		b, e = ConsoleImage(b)
		if e != nil {
			return e
		}
		return os.WriteFile(dest, b, 0o700)
	}
	if err := link(source, dest); err == nil {
		return nil
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return err
	}
	n, err := io.Copy(dst, io.LimitReader(src, maxImage+1))
	if e := dst.Close(); err == nil {
		err = e
	}
	if err == nil && n > maxImage {
		err = errors.New("kaze: Codex launcher source exceeds 64 MiB")
	}
	return err
}

func readBounded(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(b)) > limit {
		err = fmt.Errorf("kaze: %s exceeds launcher size limit", filepath.Base(path))
	}
	return b, err
}
func envValue(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		k, v, ok := strings.Cut(env[i], "=")
		if ok && strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}
func loadManifest(dir string) (manifest, error) {
	var m manifest
	b, err := readBounded(filepath.Join(dir, markerName), 4096)
	if err == nil {
		err = json.Unmarshal(b, &m)
	}
	if err == nil && (m.Version != 1 || len(m.Token) != 32 || !filepath.IsAbs(m.Source)) {
		err = errors.New("invalid launcher marker")
	}
	return m, err
}
func shimDir(dir string) bool {
	if !strings.HasPrefix(filepath.Base(dir), directoryPrefix) {
		return false
	}
	_, err := loadManifest(dir)
	return err == nil
}

// Env changes only a child shell environment. Nested Kaze copies replace the
// inherited launcher directory and identity without touching the outer shell.
func (l *Launcher) Env(env []string) []string {
	sep := string(os.PathListSeparator)
	paths := []string{l.dir}
	for _, p := range strings.Split(envValue(env, "PATH"), sep) {
		if p != l.dir && !shimDir(p) {
			paths = append(paths, p)
		}
	}
	out := make([]string, 0, len(env)+3)
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		if !strings.EqualFold(k, "PATH") && !strings.EqualFold(k, markerEnv) && !strings.EqualFold(k, tokenEnv) && !strings.EqualFold(k, depthEnv) {
			out = append(out, e)
		}
	}
	return append(l.zshEnv(env, out), "PATH="+strings.Join(paths, sep), markerEnv+"="+l.dir, tokenEnv+"="+l.token, depthEnv+"=0")
}

func (l *Launcher) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		m, err := loadManifest(l.dir)
		if err != nil || m.Token != l.token {
			l.closeErr = errors.New("kaze: refusing to remove a changed Codex launcher directory")
			return
		}
		l.closeErr = removeOwned(l.dir)
	})
	return l.closeErr
}

func validate(self string, env []string) error {
	dir := envValue(env, markerEnv)
	m, err := loadManifest(dir)
	parent, pErr := os.Stat(filepath.Dir(self))
	expected, eErr := os.Stat(dir)
	baseOK := filepath.Base(self) == shimName(runtime.GOOS)
	if runtime.GOOS == "windows" {
		baseOK = strings.EqualFold(filepath.Base(self), shimName(runtime.GOOS))
	}
	if err != nil || pErr != nil || eErr != nil || !os.SameFile(parent, expected) || dir == "" || !baseOK || m.Token != envValue(env, tokenEnv) {
		return errors.New("kaze: invalid private Codex launcher identity")
	}
	if envValue(env, "KAZE_TAB_ID") == "" {
		return errors.New("kaze: Codex launcher needs its owning Kaze tab environment")
	}
	return nil
}
