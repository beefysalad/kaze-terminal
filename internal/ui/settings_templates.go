package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/beefysalad/kaze-terminal/internal/orchestrate"
)

type templateSettings struct {
	catalog orchestrate.TemplateCatalog
	open    func(string) error // tests inject the platform opener
	openErr string
}

// Resolve from the already injected config path, never from process home/env.
func (win *window) templateFolder() string {
	if win.cfgPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(win.cfgPath), "templates")
}
func (win *window) loadTemplates() {
	win.set.templates.catalog = orchestrate.ListTemplates(win.templateFolder())
}
func (win *window) templateRows() []settingRow {
	st := &win.set.templates
	rows := []settingRow{{heading: "Templates"}}
	rows = append(rows, settingRow{label: "Worker templates", hint: "Markdown instructions for a worker. Files are read again each time a worker starts.", ctrls: []ctrl{{kind: ctlButton, id: "templates_open", text: "Open folder", dead: win.templateFolder() == ""}}})
	if len(st.catalog.Templates) == 0 {
		rows = append(rows, settingRow{label: "No templates yet"})
	}
	for _, t := range st.catalog.Templates {
		hint := t.Agent
		if t.Description != "" {
			hint += " - " + t.Description
		}
		rows = append(rows, settingRow{label: t.Name, hint: hint})
	}
	if len(st.catalog.Warnings) > 0 {
		rows = append(rows, settingRow{label: "Some templates could not be fully loaded", hint: strings.Join(st.catalog.Warnings, "; "), pill: "Check files", tone: "warn"})
	}
	if st.openErr != "" {
		rows = append(rows, settingRow{label: "Could not open templates folder", hint: st.openErr, pill: "Error", tone: "err"})
	}
	return rows
}
func (win *window) openTemplates() {
	st := &win.set.templates
	st.openErr = ""
	folder := win.templateFolder()
	if folder == "" {
		st.openErr = "Config folder unavailable"
		return
	}
	if err := os.MkdirAll(folder, 0700); err != nil {
		st.openErr = err.Error()
		return
	}
	opener := st.open
	if opener == nil {
		opener = openURL
	}
	if err := opener(folder); err != nil {
		st.openErr = err.Error()
	}
	win.loadTemplates()
}
