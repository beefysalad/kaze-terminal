package codexlaunch

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Command struct {
	Path   string
	Prefix []string
}

// Resolve uses the invoking shell's current PATH rather than Kaze's startup
// PATH. It skips all private launch directories and identical launcher copies.
func Resolve(env []string, self, goos string) (Command, error) {
	sep := ":"
	if goos == "windows" {
		sep = ";"
	}
	dirs := strings.Split(envValue(env, "PATH"), sep)
	extensions := []string{""}
	if goos == "windows" {
		ext := envValue(env, "PATHEXT")
		if ext == "" {
			ext = ".COM;.EXE;.BAT;.CMD"
		}
		extensions = strings.Split(ext, ";")
	}
	selfInfo, err := os.Stat(self)
	if err != nil {
		return Command{}, err
	}
	var selfHash [32]byte
	hashed := false
	for i, dir := range dirs {
		if i == 256 {
			return Command{}, errors.New("kaze: Codex PATH search exceeds 256 directories")
		}
		dir = strings.Trim(dir, `"`)
		if dir == "" {
			dir = "."
		}
		if shimDir(dir) {
			continue
		}
		for _, ext := range extensions {
			candidate := filepath.Join(dir, "codex"+strings.ToLower(ext))
			st, e := os.Stat(candidate)
			if e != nil || !st.Mode().IsRegular() || (goos != "windows" && st.Mode()&0o111 == 0) {
				continue
			}
			abs, e := filepath.Abs(candidate)
			if e != nil {
				continue
			}
			if os.SameFile(selfInfo, st) {
				continue
			}
			if m, e := loadManifest(envValue(env, markerEnv)); e == nil {
				if src, e := os.Stat(m.Source); e == nil && os.SameFile(src, st) {
					continue
				}
			}
			if st.Size() == selfInfo.Size() && st.Size() <= maxImage {
				if !hashed {
					selfHash, e = fileHash(self)
					if e != nil {
						return Command{}, e
					}
					hashed = true
				}
				if h, e := fileHash(abs); e == nil && h == selfHash {
					continue
				}
			}
			if goos == "windows" && (strings.EqualFold(ext, ".cmd") || strings.EqualFold(ext, ".bat")) {
				return npmCommand(abs, env)
			}
			return Command{Path: abs}, nil
		}
	}
	return Command{}, errors.New("kaze: cannot find the real Codex on this shell's PATH (private launcher copies are skipped)")
}

func fileHash(path string) ([32]byte, error) {
	var out [32]byte
	f, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, io.LimitReader(f, maxImage+1))
	copy(out[:], h.Sum(nil))
	return out, err
}

// These exact templates are npm/cmd-shim's Node wrapper, with no shebang args
// or injected environment. An adjacent package alone cannot authenticate a
// custom script: only a recognized script is replaced with direct Node argv.
const npmHead = "@ECHO off\nGOTO start\n:find_dp0\nSET dp0=%~dp0\nEXIT /b\n:start\nSETLOCAL\nCALL :find_dp0\n\nIF EXIST \"%dp0%\\node.exe\" (\n  SET \"_prog=%dp0%\\node.exe\"\n) ELSE (\n  SET \"_prog=node\"\n)\n\n"
const npmTail = "endLocal & goto #_undefined_# 2>NUL || title %COMSPEC% & set PATHEXT=%PATHEXT:;.JS;=;% & \"%_prog%\"  \"%dp0%\\node_modules\\@openai\\codex\\bin\\codex.js\" %*\n"
const npmCanonical = npmHead + npmTail
const npmLegacy = "@ECHO off\nGOTO start\n:find_dp0\nSET dp0=%~dp0\nEXIT /b\n:start\nSETLOCAL\nCALL :find_dp0\n\nIF EXIST \"%dp0%\\node.exe\" (\n  SET \"_prog=%dp0%\\node.exe\"\n) ELSE (\n  SET \"_prog=node\"\n  SET PATHEXT=%PATHEXT:;.JS;=;%\n)\n\nendLocal & goto #_undefined_# 2>NUL || title %COMSPEC% & \"%_prog%\"  \"%dp0%\\node_modules\\@openai\\codex\\bin\\codex.js\" %*\n"

func npmCommand(wrapper string, env []string) (Command, error) {
	bad := fmt.Errorf("kaze: unsupported Codex command wrapper %s; use the official npm installation or a native codex.exe", wrapper)
	b, err := readBounded(wrapper, 16<<10)
	if err != nil {
		return Command{}, bad
	}
	text := strings.TrimSpace(strings.ReplaceAll(string(b), "\r\n", "\n"))
	if text != strings.TrimSpace(npmCanonical) && text != strings.TrimSpace(npmLegacy) {
		return Command{}, bad
	}
	dir := filepath.Dir(wrapper)
	pkg := filepath.Join(dir, "node_modules", "@openai", "codex")
	b, err = readBounded(filepath.Join(pkg, "package.json"), 64<<10)
	if err != nil {
		return Command{}, bad
	}
	var meta struct {
		Name string
		Bin  map[string]string
	}
	if json.Unmarshal(b, &meta) != nil || meta.Name != "@openai/codex" || meta.Bin["codex"] != "bin/codex.js" {
		return Command{}, bad
	}
	js := filepath.Join(pkg, "bin", "codex.js")
	if st, e := os.Stat(js); e != nil || !st.Mode().IsRegular() {
		return Command{}, bad
	}
	node := filepath.Join(dir, "node.exe")
	if st, e := os.Stat(node); e != nil || !st.Mode().IsRegular() {
		node = ""
		for i, p := range strings.Split(envValue(env, "PATH"), ";") {
			if i == 256 {
				break
			}
			if shimDir(p) {
				continue
			}
			n := filepath.Join(strings.Trim(p, `"`), "node.exe")
			if st, e := os.Stat(n); e == nil && st.Mode().IsRegular() {
				node = n
				break
			}
		}
	}
	if node == "" {
		return Command{}, errors.New("kaze: official npm Codex needs node.exe on this shell's PATH")
	}
	node, err = filepath.Abs(node)
	if err != nil {
		return Command{}, err
	}
	return Command{Path: node, Prefix: []string{js}}, nil
}
