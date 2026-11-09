// Package devbox reads the status of a person's devbox jobs for the Settings
// page. It only ever reads: it starts, stops and answers nothing.
//
// Kaze ships with no key and no address. Both come from the credentials file
// that /devbox:setup writes on a paired machine; without it there is no page.
package devbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	maxCredsBytes = 8 << 10
	maxReplyBytes = 1 << 20
	maxJobs       = 500
	maxNameRunes  = 160
)

// ErrRejected means the devbox refused the key: this machine is no longer paired.
var ErrRejected = errors.New("devbox: key rejected")

// Creds is one machine's pairing. Key is never logged, drawn or put in an error.
type Creds struct {
	URL, Key, Name string
}

// Load reads this machine's pairing; ok is false when it isn't paired.
func Load() (Creds, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Creds{}, false
	}
	return LoadFile(filepath.Join(home, ".config", "devbox", "credentials"))
}

// LoadFile reads a credentials file; anything incomplete or unsafe means not paired.
func LoadFile(path string) (Creds, bool) {
	f, err := os.Open(path)
	if err != nil {
		return Creds{}, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCredsBytes+1))
	if err != nil || len(data) > maxCredsBytes {
		return Creds{}, false
	}
	var c Creds
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "DEVBOX_URL":
			c.URL = strings.TrimRight(v, "/")
		case "DEVBOX_API_KEY":
			c.Key = v
		case "DEVBOX_NAME":
			c.Name = clean(v)
		}
	}
	if c.Key == "" || !safeURL(c.URL) {
		return Creds{}, false
	}
	return c, true
}

// safeURL reports whether a key may be sent to raw.
func safeURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

// Status is a job's state.
type Status int

const (
	Unknown Status = iota
	Running
	NeedsYou
	Done
	Stopped
	Failed
	Queued
)

var statuses = map[string]Status{
	"running": Running, "needs_you": NeedsYou, "done": Done, "stopped": Stopped, "failed": Failed, "queued": Queued,
}

// Job is one line of the devbox's list.
type Job struct {
	ID       string
	Name     string
	Status   Status
	Progress int // 0..100, -1 when the devbox gave none
	Age      time.Duration
	AgeText  string // as the devbox wrote it, "84m"
	Question string // what a job that needs you is asking
}

var (
	idRe      = regexp.MustCompile(`^j[0-9a-f]{3,}(\s|$)`)
	lineRe    = regexp.MustCompile(`^(j[0-9a-f]+)\s+(\S+)\s+(\S+)\s+(\d+)([smhd])\s+(.*)$`)
	modelRe   = regexp.MustCompile(`^\[[^\]]*\]\s*`)
	lapRe     = regexp.MustCompile(`^(?:\w+ lap )?(lap-[^\s:]+)(?::\s|$)`)
	percentRe = regexp.MustCompile(`^(\d{1,3})%`)
)

// Parse reads the list_agents text. Lines that aren't jobs, such as notices,
// are skipped. A job it can't read shows as Unknown; it never guesses a status.
func Parse(text string) []Job {
	var jobs []Job
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(clean(raw))
		if !idRe.MatchString(line) {
			continue
		}
		jobs = append(jobs, parseLine(line))
		if len(jobs) == maxJobs {
			break
		}
	}
	return jobs
}

func parseLine(line string) Job {
	m := lineRe.FindStringSubmatch(line)
	if m == nil {
		id, _, _ := strings.Cut(line, " ")
		return Job{ID: id, Name: limit(line), Progress: -1}
	}
	j := Job{ID: m[1], Status: statuses[m[2]], Progress: -1, AgeText: m[4] + m[5]}
	n, _ := strconv.Atoi(m[4])
	j.Age = time.Duration(n) * map[string]time.Duration{"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}[m[5]]
	title, details, _ := strings.Cut(modelRe.ReplaceAllString(m[6], ""), "  ")
	details = strings.TrimSpace(details)
	j.Name = limit(lapName(strings.TrimSpace(title)))
	if q, ok := strings.CutPrefix(details, "needs you: "); ok {
		j.Question = q
	}
	if p := percentRe.FindStringSubmatch(details); p != nil {
		v, _ := strconv.Atoi(p[1])
		j.Progress = min(v, 100)
	}
	return j
}

// lapName shortens both forms a lap's title comes in to the lap's id.
func lapName(title string) string {
	if m := lapRe.FindStringSubmatch(title); m != nil {
		return m[1]
	}
	return title
}

func limit(s string) string {
	if r := []rune(s); len(r) > maxNameRunes {
		return string(r[:maxNameRunes])
	}
	return s
}

// clean removes control and invisible formatting characters.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}

// Groups are the page's sections.
type Groups struct {
	NeedsYou, Running, Queued, Finished []Job
	FailedToday                         int
}

// Group sorts jobs into the page's sections, newest first where order matters.
func Group(jobs []Job) Groups {
	var g Groups
	for _, j := range jobs {
		switch j.Status {
		case NeedsYou:
			g.NeedsYou = append(g.NeedsYou, j)
		case Running:
			g.Running = append(g.Running, j)
		case Queued:
			g.Queued = append(g.Queued, j)
		default:
			g.Finished = append(g.Finished, j)
			if j.Status == Failed && j.Age < 24*time.Hour {
				g.FailedToday++
			}
		}
	}
	newest := func(a, b Job) int { return int(a.Age - b.Age) }
	slices.SortStableFunc(g.Running, newest)
	slices.SortStableFunc(g.Finished, newest)
	return g
}

// List asks the devbox for the jobs. ctx bounds the whole exchange; hc may be nil.
func List(ctx context.Context, c Creds, hc *http.Client) ([]Job, error) {
	if !safeURL(c.URL) {
		return nil, errors.New("devbox: address is not https")
	}
	s := &mcpSession{creds: c, http: &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
	if hc != nil {
		s.http.Transport = hc.Transport
	}
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	err := s.call(ctx, 1, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "kaze", "version": "1"},
	}, &init)
	if err != nil {
		return nil, err
	}
	s.version = init.ProtocolVersion
	if err := s.call(ctx, 0, "notifications/initialized", nil, nil); err != nil {
		return nil, err
	}
	var out struct {
		IsError bool `json:"isError"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	err = s.call(ctx, 2, "tools/call", map[string]any{"name": "list_agents", "arguments": map[string]any{}}, &out)
	if err != nil {
		return nil, err
	}
	if out.IsError {
		return nil, errors.New("devbox: the job list failed")
	}
	var text strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
			text.WriteByte('\n')
		}
	}
	return Parse(text.String()), nil
}

// mcpSession is the small part of MCP over streamable HTTP that listing needs.
type mcpSession struct {
	creds   Creds
	http    *http.Client
	session string
	version string
}

// call sends one message. id 0 is a notification: no reply is read.
func (s *mcpSession) call(ctx context.Context, id int, method string, params, into any) error {
	msg := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != 0 {
		msg["id"] = id
	}
	if params != nil {
		msg["params"] = params
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.creds.URL+"/mcp", bytes.NewReader(body))
	if err != nil {
		return errors.New("devbox: bad address")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+s.creds.Key)
	if s.session != "" {
		req.Header.Set("Mcp-Session-Id", s.session)
	}
	if s.version != "" {
		req.Header.Set("MCP-Protocol-Version", s.version)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("devbox: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrRejected
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("devbox: replied %d", resp.StatusCode)
	}
	if v := resp.Header.Get("Mcp-Session-Id"); v != "" {
		s.session = v
	}
	if id == 0 {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes+1))
	if err != nil {
		return fmt.Errorf("devbox: %w", err)
	}
	if len(data) > maxReplyBytes {
		return errors.New("devbox: reply too large")
	}
	return decodeReply(data, id, into)
}

type rpcReply struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code int `json:"code"`
	} `json:"error"`
}

// decodeReply finds message id in a JSON or event-stream reply.
func decodeReply(data []byte, id int, into any) error {
	payloads := [][]byte{data}
	if t := bytes.TrimSpace(data); !bytes.HasPrefix(t, []byte("{")) {
		payloads = nil
		sc := bufio.NewScanner(bytes.NewReader(data))
		sc.Buffer(make([]byte, 0, 64<<10), maxReplyBytes+1)
		for sc.Scan() {
			if p, ok := bytes.CutPrefix(sc.Bytes(), []byte("data:")); ok {
				payloads = append(payloads, slices.Clone(bytes.TrimSpace(p)))
			}
		}
	}
	for _, p := range payloads {
		var r rpcReply
		if json.Unmarshal(p, &r) != nil || r.ID != id {
			continue
		}
		if r.Error != nil {
			return fmt.Errorf("devbox: error %d", r.Error.Code)
		}
		if json.Unmarshal(r.Result, into) != nil {
			return errors.New("devbox: unreadable reply")
		}
		return nil
	}
	return errors.New("devbox: no reply")
}
