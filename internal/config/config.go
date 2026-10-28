// Package config reads kaze's single TOML settings file. There is no
// settings UI, so bad values degrade to defaults and come back as warnings.
package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	defaultFontSize   = 14
	minFontSize       = 4
	maxFontSize       = 72
	defaultScrollback = 10000
	maxScrollback     = 100000
)

type Config struct {
	FontSize   float32 `toml:"font_size"`   // default 14
	FontFamily string  `toml:"font_family"` // not supported yet; a set value warns
	// Shell empty means "use the platform default shell"; the caller
	// resolves it at startup.
	Shell           []string          `toml:"shell"`
	ScrollbackLines int               `toml:"scrollback_lines"` // default 10000, max 100000
	Colors          map[string]string `toml:"colors"`           // "fg", "bg", "cursor", "selection", "color0".."color15": "#rrggbb"
	Keys            map[string]string `toml:"keys"`             // action -> shortcut text; parsed by the caller
	CopyOnSelect    bool              `toml:"copy_on_select"`   // copy a selection when the mouse is released
	ChimeNeeds      bool              `toml:"chime_needs"`      // chime when Claude needs you; default true
	ChimeDone       bool              `toml:"chime_done"`       // chime when Claude is done; default false
	ChimeFile       string            `toml:"chime_file"`       // custom WAV; ~ expands to home, other relative paths use the working dir; empty = bundled chime
}

// KnownActions are the action names accepted under [keys]; the window binds each one.
var KnownActions = map[string]bool{
	"new_tab": true, "close_tab": true, "next_tab": true, "prev_tab": true,
	"copy": true, "paste": true,
}

var (
	hexColor  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	colorKeys = func() map[string]bool {
		m := map[string]bool{"fg": true, "bg": true, "cursor": true, "selection": true}
		for i := 0; i < 16; i++ {
			m[fmt.Sprintf("color%d", i)] = true
		}
		return m
	}()
)

func Default() Config {
	return Config{
		FontSize:        defaultFontSize,
		ScrollbackLines: defaultScrollback,
		ChimeNeeds:      true,
		Colors:          map[string]string{},
		Keys:            map[string]string{},
	}
}

// Path returns <UserConfigDir>/kaze/config.toml, or "" if there is no config dir.
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "kaze", "config.toml")
}

// Load reads path. A missing file gives defaults; a TOML syntax error is an
// error; bad values are replaced by defaults and reported as warnings.
func Load(path string) (Config, []string, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil, nil
	}
	if err != nil {
		return cfg, nil, err
	}
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return Default(), nil, fmt.Errorf("config %s: %w", path, err)
	}

	var warns []string
	warn := func(f string, a ...any) { warns = append(warns, fmt.Sprintf(f, a...)) }

	for _, k := range sortedKeys(raw) {
		v := raw[k]
		switch k {
		case "font_size":
			f, ok := toFloat(v)
			switch {
			case !ok || math.IsNaN(f) || math.IsInf(f, 0) || f <= 0:
				warn("font_size must be a positive number, got %v; using %d", v, defaultFontSize)
			case f < minFontSize:
				warn("font_size %v is too small; raised to %d", v, minFontSize)
				cfg.FontSize = minFontSize
			case f > maxFontSize:
				warn("font_size %v is too large; capped at %d", v, maxFontSize)
				cfg.FontSize = maxFontSize
			default:
				cfg.FontSize = float32(f)
			}
		case "font_family":
			s, ok := v.(string)
			if !ok {
				warn("font_family must be a string, got %v; using the bundled font", v)
				continue
			}
			cfg.FontFamily = s
			// Kept for when font loading lands; until then only the bundled font renders.
			if s != "" {
				warn("font_family is not supported yet; using the bundled font")
			}
		case "shell":
			if s, ok := toStrings(v); ok && strings.TrimSpace(s[0]) != "" {
				cfg.Shell = s
			} else {
				warn("shell must be a list of strings starting with a program, got %v; using the default shell", v)
			}
		case "scrollback_lines":
			n, ok := v.(int64)
			switch {
			case !ok || n <= 0:
				warn("scrollback_lines must be a positive integer, got %v; using %d", v, defaultScrollback)
			case n > maxScrollback:
				warn("scrollback_lines %d is too large; capped at %d", n, maxScrollback)
				cfg.ScrollbackLines = maxScrollback
			default:
				cfg.ScrollbackLines = int(n)
			}
		case "copy_on_select":
			b, ok := v.(bool)
			if !ok {
				warn("copy_on_select must be true or false, got %v; using false", v)
				continue
			}
			cfg.CopyOnSelect = b
		case "chime_needs":
			b, ok := v.(bool)
			if !ok {
				warn("chime_needs must be true or false, got %v; using true", v)
				continue
			}
			cfg.ChimeNeeds = b
		case "chime_done":
			b, ok := v.(bool)
			if !ok {
				warn("chime_done must be true or false, got %v; using false", v)
				continue
			}
			cfg.ChimeDone = b
		case "chime_file":
			s, ok := v.(string)
			if !ok {
				warn("chime_file must be a path string, got %v; using the bundled chime", v)
				continue
			}
			s = expandHome(s)
			if s != "" {
				if st, err := os.Stat(s); err != nil || st.IsDir() {
					warn("chime_file %q is not a readable file; using the bundled chime", s)
					continue
				}
			}
			cfg.ChimeFile = s
		case "colors":
			loadColors(v, &cfg, warn)
		case "keys":
			loadKeys(v, &cfg, warn)
		default:
			warn("unknown setting %q ignored", k)
		}
	}
	return cfg, warns, nil
}

func loadColors(v any, cfg *Config, warn func(string, ...any)) {
	t, ok := v.(map[string]any)
	if !ok {
		warn("colors must be a table; ignored")
		return
	}
	for _, k := range sortedKeys(t) {
		s, isStr := t[k].(string)
		switch {
		case !colorKeys[k]:
			warn("unknown color %q ignored", k)
		case !isStr || !hexColor.MatchString(s):
			warn("color %s must look like \"#rrggbb\", got %v; using the default", k, t[k])
		default:
			cfg.Colors[k] = s
		}
	}
}

func loadKeys(v any, cfg *Config, warn func(string, ...any)) {
	t, ok := v.(map[string]any)
	if !ok {
		warn("keys must be a table; ignored")
		return
	}
	for _, k := range sortedKeys(t) {
		s, isStr := t[k].(string)
		switch {
		case !KnownActions[k]:
			warn("unknown key action %q ignored", k)
		case !isStr || s == "":
			warn("shortcut for %s must be a non-empty string, got %v; using the default", k, t[k])
		default:
			cfg.Keys[k] = s
		}
	}
}

// expandHome turns a leading ~ into the home dir. Other relative paths are
// left alone and resolve against the working directory.
func expandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") && !strings.HasPrefix(p, "~\\") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[1:])
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func toStrings(v any) ([]string, bool) {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil, false
	}
	out := make([]string, len(list))
	for i, e := range list {
		s, ok := e.(string)
		if !ok {
			return nil, false
		}
		out[i] = s
	}
	return out, true
}

// Sorted so warnings come out in a stable order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
