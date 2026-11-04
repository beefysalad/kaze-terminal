package main

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/beefysalad/kaze-terminal/internal/notify"
)

var patchHeaders = []string{"*** Add File: ", "*** Update File: ", "*** Move to: "}

// docPaths lists the absolute markdown paths a tool call touches (at most
// notify.MaxDocs): file_path and path of any tool, and the file headers of a
// Codex apply_patch. Relative paths are joined to cwd; with no cwd they drop.
func docPaths(tool string, input json.RawMessage, cwd string) []string {
	var in struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
		Command  string `json:"command"`
	}
	if json.Unmarshal(input, &in) != nil {
		return nil
	}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if len(out) >= notify.MaxDocs || !strings.EqualFold(filepath.Ext(p), ".md") {
			return
		}
		if !filepath.IsAbs(p) {
			if cwd == "" {
				return
			}
			p = filepath.Join(cwd, p)
		}
		p = filepath.Clean(p)
		if len(p) > notify.MaxPath {
			return
		}
		out = append(out, p)
	}
	add(in.FilePath)
	add(in.Path)
	if tool == "apply_patch" {
		for _, line := range strings.Split(in.Command, "\n") {
			line = strings.TrimSpace(line)
			for _, h := range patchHeaders {
				if f, ok := strings.CutPrefix(line, h); ok {
					add(f)
				}
			}
		}
	}
	return out
}
