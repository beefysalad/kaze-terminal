package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ctl "github.com/beefysalad/kaze-terminal/internal/control"
)

func installTestTemplate(t *testing.T, win *window, name, text string) string {
	t.Helper()
	if win.cfgPath == "" {
		win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
	}
	dir := filepath.Join(filepath.Dir(win.cfgPath), "templates")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestTemplateSettingsCachedAndOpen(t *testing.T) {
	win := sheetWindow(t, "midnight")
	win.selectSection(claudeIndex(t))
	dir := filepath.Join(filepath.Dir(win.cfgPath), "templates")
	win.loadAgents()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("created folder on load", err)
	}
	win.drawSettings(pageContext(900, 700), pageRect)
	if !pageHas(win, "No templates yet") {
		t.Fatal(win.set.texts)
	}
	path := installTestTemplate(t, win, "review", "---\nagent: codex\ndescription: Review changes\n---\nKeep tests.")
	win.loadAgents()
	win.drawSettings(pageContext(900, 700), pageRect)
	if !pageHas(win, "review") || !pageHas(win, "codex - Review changes") {
		t.Fatal(win.set.texts)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	win.drawSettings(pageContext(900, 700), pageRect)
	if !pageHas(win, "review") {
		t.Fatal("read folder on frame")
	}
	var opened string
	win.set.templates.open = func(s string) error { opened = s; return nil }
	agentsAct(win, "templates_open", 0)
	if opened != dir {
		t.Fatal(opened)
	}
	win.drawSettings(pageContext(900, 700), pageRect)
	if !pageHas(win, "No templates yet") {
		t.Fatal("did not refresh on Open")
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	agentsAct(win, "templates_open", 0)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatal("did not create lazily", err)
	}
}
func TestTemplateWorkerSpawnAndList(t *testing.T) {
	win, _ := workerWindow(t)
	win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
	installTestTemplate(t, win, "review", "---\nagent: codex\nmodel: gpt-5.1\nmode: read-only\nflags: --yolo\n---\nKeep tests.")
	r := win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "missing", Task: "Review"})
	if r.Error == nil || r.Error.Code != "unknown_template" {
		t.Fatal(r)
	}
	r = win.applyWorker(ctl.Request{Op: "spawn", From: "t1", Agent: "review", Mode: "edit", Task: "Review"})
	if !r.OK || len(r.Tabs) != 1 || r.Tabs[0].Agent != "codex" || !strings.Contains(r.Warning, "flags") {
		t.Fatal(r)
	}
	r = win.applyWorker(ctl.Request{Op: "templates"})
	if !r.OK || r.Text != "review  codex  " || !strings.Contains(r.Warning, "flags") {
		t.Fatal(r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r = win.control.Templates(ctx); r.Error == nil {
		t.Fatal("ignored cancellation")
	}
}
func TestTemplateConsentLabel(t *testing.T) {
	for _, tt := range []struct{ agent, want string }{{"codex", "Codex (review)"}, {"claude", "Claude Code (review)"}, {"shell", "review"}} {
		win, _ := workerWindow(t)
		win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
		win.opt.Config.Orchestrator.Confirm = "always"
		installTestTemplate(t, win, "review", "---\nagent: "+tt.agent+"\n---\nReview")
		r := consentRequest("t1")
		r.req.Agent = "review"
		if !win.holdWorkerConsent(r) {
			t.Fatal("not held")
		}
		c := win.consent.pending[1]
		if !strings.Contains(c.title, "a "+tt.want+" worker") {
			t.Fatal(c.title)
		}
	}
}

func TestTemplateListQueuedOnUI(t *testing.T) {
	win, _ := workerWindow(t)
	win.cfgPath = filepath.Join(t.TempDir(), "config.toml")
	path := installTestTemplate(t, win, "review", "---\nagent: codex\n---\nReview")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan ctl.Reply, 1)
	woke := make(chan struct{}, 1)
	win.control.wake = func() {
		select {
		case woke <- struct{}{}:
		default:
		}
	}
	go func() { done <- win.control.Templates(ctx) }()
	select {
	case <-woke:
	case <-ctx.Done():
		t.Fatal("queue did not wake UI")
	}
	select {
	case r := <-done:
		t.Fatal("listed outside UI", r)
	default:
	}
	if err := os.WriteFile(path, []byte("---\nagent: claude\n---\nReview"), 0600); err != nil {
		t.Fatal(err)
	}
	win.drainWorkers()
	select {
	case r := <-done:
		if !r.OK || r.Text != "review  claude  " {
			t.Fatal(r)
		}
	case <-ctx.Done():
		t.Fatal("UI did not return list")
	}
}
