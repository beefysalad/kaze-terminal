package devbox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const secret = "dbx-super-secret-key"

func writeCreds(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "credentials")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadCreds(t *testing.T) {
	good := "DEVBOX_URL=https://box.example\nDEVBOX_API_KEY=" + secret + "\nDEVBOX_NAME=alex\n"
	cases := []struct {
		name, body string
		ok         bool
	}{
		{"valid", good, true},
		{"windows line endings", strings.ReplaceAll(good, "\n", "\r\n"), true},
		{"no key", "DEVBOX_URL=https://box.example\nDEVBOX_NAME=p\n", false},
		{"empty key", "DEVBOX_URL=https://box.example\nDEVBOX_API_KEY=\n", false},
		{"no url", "DEVBOX_API_KEY=k\n", false},
		{"plain http", "DEVBOX_URL=http://box.example\nDEVBOX_API_KEY=k\n", false},
		{"no host", "DEVBOX_URL=https://\nDEVBOX_API_KEY=k\n", false},
		{"user in url", "DEVBOX_URL=https://a:b@box.example\nDEVBOX_API_KEY=k\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := LoadFile(writeCreds(t, c.body))
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if ok && (got.URL != "https://box.example" || got.Key != secret || got.Name != "alex") {
				t.Fatalf("creds = %+v", got)
			}
		})
	}
}

func TestLoadMissingOrOversizedFile(t *testing.T) {
	if _, ok := LoadFile(filepath.Join(t.TempDir(), "nope")); ok {
		t.Fatal("a missing file must mean not paired")
	}
	big := "DEVBOX_URL=https://box.example\nDEVBOX_API_KEY=k\n" + strings.Repeat("x", maxCredsBytes+1)
	if _, ok := LoadFile(writeCreds(t, big)); ok {
		t.Fatal("an oversized file must mean not paired")
	}
}

func TestParseEveryStatus(t *testing.T) {
	in := strings.Join([]string{
		"j4d0ce4  running  lead    84m  [claude opus] Batch lap lap-1140-b  40% · ~2h · ticket 1",
		"j6ac836  failed   lead    88m  [claude opus] Batch lap lap-1136-b",
		"j5dd154  stopped  lead    24h  [claude opus] lap-1606: Run the batch  10% · ~1h30m",
		"ja15391  done     lead    78h  [claude opus] lap-0700: Run the batch  100% · stage 10",
		"jd561e8  needs_you lead   138h  [claude opus] lap-1446: Run the batch  needs you: OK to push? item 12 Ready",
		"j68770c  stopped  worker 116h  [claude opus] Fleet DNS check",
		"j00b7bf  done     lead   194h  lap-1708: Run the batch  100% · ticket 2",
	}, "\n")
	jobs := Parse(in)
	if len(jobs) != 7 {
		t.Fatalf("got %d jobs, want 7", len(jobs))
	}
	want := []struct {
		st       Status
		name     string
		progress int
		age      time.Duration
	}{
		{Running, "lap-1140-b", 40, 84 * time.Minute},
		{Failed, "lap-1136-b", -1, 88 * time.Minute},
		{Stopped, "lap-1606", 10, 24 * time.Hour},
		{Done, "lap-0700", 100, 78 * time.Hour},
		{NeedsYou, "lap-1446", -1, 138 * time.Hour},
		{Stopped, "Fleet DNS check", -1, 116 * time.Hour},
		{Done, "lap-1708", 100, 194 * time.Hour},
	}
	for i, w := range want {
		j := jobs[i]
		if j.Status != w.st || j.Name != w.name || j.Progress != w.progress || j.Age != w.age {
			t.Errorf("job %d = %+v, want %+v", i, j, w)
		}
	}
	if jobs[4].Question != "OK to push? item 12 Ready" {
		t.Errorf("question = %q", jobs[4].Question)
	}
	if jobs[0].AgeText != "84m" {
		t.Errorf("age text = %q", jobs[0].AgeText)
	}
}

func TestQueuedJobsAreNotFinished(t *testing.T) {
	jobs := Parse("j7dcb67  queued   lead     5m  [claude opus] Batch lap lap-q  waiting for room, 2 in line")
	if len(jobs) != 1 || jobs[0].Status != Queued || jobs[0].Name != "lap-q" {
		t.Fatalf("jobs = %+v", jobs)
	}
	g := Group(jobs)
	if len(g.Queued) != 1 || len(g.Finished) != 0 {
		t.Fatalf("groups = %+v", g)
	}
}

func TestListUnreadableFormatIsAnError(t *testing.T) {
	creds, hc := startFake(t, &fakeMCP{reply: "Jobs:\n| id | state |\n| abc | working |"})
	if _, err := List(bounded(t), creds, hc); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("err = %v, want ErrUnreadable: a changed format must not look like no jobs", err)
	}
	creds, hc = startFake(t, &fakeMCP{reply: "  \n"})
	if jobs, err := List(bounded(t), creds, hc); err != nil || len(jobs) != 0 {
		t.Fatalf("an empty list is just empty: %v, %v", jobs, err)
	}
}

func TestQuestionIsCapped(t *testing.T) {
	jobs := Parse("jabc123  needs_you  lead  3m  A lap  needs you: " + strings.Repeat("q", maxQuestionRunes+200))
	if len(jobs) != 1 || len([]rune(jobs[0].Question)) != maxQuestionRunes {
		t.Fatalf("question is %d runes, want the cap %d", len([]rune(jobs[0].Question)), maxQuestionRunes)
	}
}

func TestParseOddLines(t *testing.T) {
	if got := Parse(""); len(got) != 0 {
		t.Fatalf("empty reply gave %d jobs", len(got))
	}
	// A notice beside the list is not a job.
	if got := Parse("A newer devbox plugin is available.\n\nNo jobs yet"); len(got) != 0 {
		t.Fatalf("notices became jobs: %+v", got)
	}
	got := Parse("jabc123  teleporting  lead  3m  Mystery job")
	if len(got) != 1 || got[0].Status != Unknown || got[0].Name != "Mystery job" {
		t.Fatalf("unknown status = %+v", got)
	}
	// A job id with the rest of the line unreadable still shows, and is never given a status.
	got = Parse("jabc123")
	if len(got) != 1 || got[0].Status != Unknown || got[0].Name != "jabc123" {
		t.Fatalf("unreadable line = %+v", got)
	}
	got = Parse("jabc123  running  lead  3m  Bad\x1b[31m‮Name\x07  140%")
	if len(got) != 1 || got[0].Name != "Bad[31mName" {
		t.Fatalf("control characters kept: %q", got[0].Name)
	}
	if got[0].Progress != 100 {
		t.Fatalf("progress = %d, want it capped at 100", got[0].Progress)
	}
}

func TestParseIsBounded(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxJobs+50; i++ {
		b.WriteString("jabc123  done  lead  3m  x\n")
	}
	if got := Parse(b.String()); len(got) != maxJobs {
		t.Fatalf("got %d jobs, want the cap %d", len(got), maxJobs)
	}
}

func TestGroup(t *testing.T) {
	jobs := []Job{
		{Name: "old run", Status: Running, Age: 4 * time.Hour},
		{Name: "new run", Status: Running, Age: time.Minute},
		{Name: "ask", Status: NeedsYou, Age: 9 * time.Hour},
		{Name: "bad recent", Status: Failed, Age: 88 * time.Minute},
		{Name: "bad old", Status: Failed, Age: 48 * time.Hour},
		{Name: "ok", Status: Done, Age: time.Hour},
		{Name: "?", Status: Unknown, Age: 3 * time.Minute},
	}
	g := Group(jobs)
	if len(g.NeedsYou) != 1 || g.NeedsYou[0].Name != "ask" {
		t.Errorf("needs you = %+v", g.NeedsYou)
	}
	if len(g.Running) != 2 || g.Running[0].Name != "new run" {
		t.Errorf("running must be newest first: %+v", g.Running)
	}
	if len(g.Finished) != 3 || g.Finished[0].Name != "ok" {
		t.Errorf("finished = %+v", g.Finished)
	}
	if len(g.Unknown) != 1 || g.Unknown[0].Name != "?" {
		t.Errorf("a job kaze can't read must not count as finished: %+v", g.Unknown)
	}
	if g.FailedToday != 1 {
		t.Errorf("failed today = %d, want 1", g.FailedToday)
	}
}

// fakeMCP is a small devbox: it records calls and answers list_agents.
type fakeMCP struct {
	mu     sync.Mutex
	calls  []string
	auth   []string
	sess   []string
	status int
	sse    bool
	reply  string
}

func (f *fakeMCP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     *int   `json:"id"`
		Method string `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	f.mu.Lock()
	label := req.Method
	if req.Method == "tools/call" {
		label += ":" + req.Params.Name
	}
	f.calls = append(f.calls, label)
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	f.sess = append(f.sess, r.Header.Get("Mcp-Session-Id"))
	status, sse, reply := f.status, f.sse, f.reply
	f.mu.Unlock()
	if status != 0 {
		w.WriteHeader(status)
		return
	}
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any = map[string]any{"protocolVersion": "2025-03-26"}
	if req.Method == "tools/call" {
		result = map[string]any{"content": []map[string]any{{"type": "text", "text": reply}}}
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": result})
	w.Header().Set("Mcp-Session-Id", "sess-1")
	if sse {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message\ndata: " + string(body) + "\n\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func startFake(t *testing.T, f http.Handler) (Creds, *http.Client) {
	t.Helper()
	s := httptest.NewTLSServer(f)
	t.Cleanup(s.Close)
	return Creds{URL: s.URL, Key: secret, Name: "p"}, s.Client()
}

func bounded(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestListHandshakeAndHeaders(t *testing.T) {
	for _, sse := range []bool{false, true} {
		f := &fakeMCP{sse: sse, reply: "jabc123  running  lead  3m  A lap  50% · x"}
		creds, hc := startFake(t, f)
		jobs, err := List(bounded(t), creds, hc)
		if err != nil {
			t.Fatalf("sse=%v: %v", sse, err)
		}
		if len(jobs) != 1 || jobs[0].Progress != 50 {
			t.Fatalf("jobs = %+v", jobs)
		}
		want := "initialize,notifications/initialized,tools/call:list_agents"
		if got := strings.Join(f.calls, ","); got != want {
			t.Fatalf("calls = %s", got)
		}
		for _, a := range f.auth {
			if a != "Bearer "+secret {
				t.Fatalf("auth header = %q", a)
			}
		}
		if f.sess[0] != "" || f.sess[2] != "sess-1" {
			t.Fatalf("session header = %q", f.sess)
		}
	}
}

func TestListRejectedKey(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		creds, hc := startFake(t, &fakeMCP{status: code})
		if _, err := List(bounded(t), creds, hc); !errors.Is(err, ErrRejected) {
			t.Fatalf("%d: err = %v, want ErrRejected", code, err)
		}
	}
}

func TestListServerErrorHidesKey(t *testing.T) {
	creds, hc := startFake(t, &fakeMCP{status: http.StatusInternalServerError})
	_, err := List(bounded(t), creds, hc)
	if err == nil || errors.Is(err, ErrRejected) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("the key leaked into an error")
	}
}

func TestListRefusesPlainHTTP(t *testing.T) {
	hit := false
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer s.Close()
	_, err := List(bounded(t), Creds{URL: s.URL, Key: secret}, s.Client())
	if err == nil || hit {
		t.Fatalf("err = %v, hit = %v: the key must never go over http", err, hit)
	}
}

func TestListRefusesRedirect(t *testing.T) {
	var followed bool
	other := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { followed = true }))
	defer other.Close()
	creds, hc := startFake(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	if _, err := List(bounded(t), creds, hc); err == nil || followed {
		t.Fatalf("err = %v, followed = %v", err, followed)
	}
}

func TestListTimesOut(t *testing.T) {
	release := make(chan struct{})
	creds, hc := startFake(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := List(ctx, creds, hc); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want a timeout error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("List did not stop at its deadline")
	}
}

func TestListCapsReply(t *testing.T) {
	creds, hc := startFake(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxReplyBytes+1)))
	}))
	if _, err := List(bounded(t), creds, hc); err == nil {
		t.Fatal("an oversized reply must be an error")
	}
}

func TestListToolError(t *testing.T) {
	creds, hc := startFake(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID int `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID,
			"result": map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": "nope"}}}})
	}))
	if _, err := List(bounded(t), creds, hc); err == nil {
		t.Fatal("a tool error must be an error")
	}
}
