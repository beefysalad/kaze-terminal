package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingFileGivesDefaults(t *testing.T) {
	cfg, warns, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(warns) != 0 {
		t.Fatalf("warnings = %v", warns)
	}
	d := Default()
	if cfg.FontSize != 14 || cfg.ScrollbackLines != 10000 || cfg.FontFamily != "" || len(cfg.Shell) != 0 {
		t.Fatalf("cfg = %+v, want defaults %+v", cfg, d)
	}
}

func TestBadValuesFallBackWithWarning(t *testing.T) {
	p := write(t, "font_size = -3\n[colors]\nfg = \"#zzz\"\n")
	cfg, warns, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FontSize != 14 {
		t.Fatalf("FontSize = %v, want 14", cfg.FontSize)
	}
	if _, ok := cfg.Colors["fg"]; ok {
		t.Fatalf("bad color kept: %v", cfg.Colors)
	}
	if len(warns) != 2 {
		t.Fatalf("warnings = %v, want 2", warns)
	}
}

func TestScrollbackIsCapped(t *testing.T) {
	cfg, warns, err := Load(write(t, "scrollback_lines = 5_000_000\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ScrollbackLines != 100000 {
		t.Fatalf("ScrollbackLines = %d", cfg.ScrollbackLines)
	}
	if len(warns) != 1 {
		t.Fatalf("warnings = %v, want 1", warns)
	}
}

func TestPartialFileKeepsOtherDefaults(t *testing.T) {
	cfg, warns, err := Load(write(t, "font_size = 18\nshell = [\"pwsh\", \"-NoLogo\"]\n[colors]\nbg = \"#1A2B3c\"\n[keys]\nnew_tab = \"ctrl+shift+t\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("warnings = %v", warns)
	}
	if cfg.FontSize != 18 || cfg.ScrollbackLines != 10000 {
		t.Fatalf("cfg = %+v", cfg)
	}
	if len(cfg.Shell) != 2 || cfg.Shell[0] != "pwsh" {
		t.Fatalf("Shell = %v", cfg.Shell)
	}
	if cfg.Colors["bg"] != "#1A2B3c" || cfg.Keys["new_tab"] != "ctrl+shift+t" {
		t.Fatalf("colors=%v keys=%v", cfg.Colors, cfg.Keys)
	}
}

func TestUnknownKeysWarn(t *testing.T) {
	cfg, warns, err := Load(write(t, "bogus = 1\n[colors]\ncolor16 = \"#000000\"\ncolor15 = \"#ffffff\"\n[keys]\nfly = \"ctrl+f\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 3 {
		t.Fatalf("warnings = %v, want 3", warns)
	}
	if cfg.Colors["color15"] != "#ffffff" || len(cfg.Colors) != 1 || len(cfg.Keys) != 0 {
		t.Fatalf("colors=%v keys=%v", cfg.Colors, cfg.Keys)
	}
}

func TestWrongTypeWarns(t *testing.T) {
	cfg, warns, err := Load(write(t, "font_size = \"big\"\nshell = \"pwsh\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FontSize != 14 || len(cfg.Shell) != 0 || len(warns) != 2 {
		t.Fatalf("cfg=%+v warns=%v", cfg, warns)
	}
}

func TestSyntaxErrorFails(t *testing.T) {
	if _, _, err := Load(write(t, "font_size = = 3\n")); err == nil {
		t.Fatal("want error")
	}
}

func TestPathEndsInKazeConfig(t *testing.T) {
	p := filepath.ToSlash(Path())
	if p != "" && !strings.HasSuffix(p, "kaze/config.toml") {
		t.Fatalf("Path = %q", p)
	}
}
