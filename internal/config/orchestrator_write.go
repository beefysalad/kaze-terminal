package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

var orchestratorHeader = regexp.MustCompile(`^\s*\[\s*["']?orchestrator["']?\s*\]\s*(#.*)?\r?$`)

// SetOrchestratorEnabled preserves unrelated lines and comments, like Set.
// Non-table forms are refused without writing, rather than rewriting owner data.
func SetOrchestratorEnabled(path string, enabled bool) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	src := string(data)
	var probe map[string]any
	if _, err := toml.Decode(src, &probe); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	lines := strings.SplitAfter(src, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if _, has := probe["orchestrator"]; has {
		if h, _ := findTable(lines, topStarts(lines), orchestratorHeader); h < 0 {
			return fmt.Errorf("config %s: workers need a [orchestrator] table", path)
		}
	}
	eol := "\n"
	if strings.Contains(src, "\r\n") {
		eol = "\r\n"
	}
	out := strings.Join(editTableLine(lines, "enabled", strconv.FormatBool(enabled), eol, "orchestrator", orchestratorHeader), "")
	if _, err := toml.Decode(out, &probe); err != nil {
		return fmt.Errorf("config %s: edit would not parse: %w", path, err)
	}
	if out == src {
		return nil
	}
	return writeFile(path, out)
}
