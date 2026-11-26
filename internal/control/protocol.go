package control

import (
	"time"

	"github.com/beefysalad/kaze-terminal/internal/orchestrate"
)

const (
	MaxKeys      = 64
	MaxTextBytes = 32 << 10
	MaxLines     = 2000 // additional scrollback rows, not visible rows
)

// Request is one control operation. The transport authenticates Token before
// calling Handle; worker management also authenticates the per-tab caller key.
type Request struct {
	Agent     string            `json:"agent,omitempty"`
	CWD       string            `json:"cwd,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Mode      string            `json:"mode,omitempty"`
	WaitStart bool              `json:"wait_start,omitempty"`
	Task      string            `json:"task,omitempty"`
	CallerKey string            `json:"caller_key,omitempty"` // separate from label Key
	From      string            `json:"from,omitempty"`
	IfIdle    bool              `json:"if_idle,omitempty"`
	Name      string            `json:"name,omitempty"`
	Key       string            `json:"key,omitempty"`
	Value     string            `json:"value,omitempty"`
	For       string            `json:"for,omitempty"`
	Timeout   int               `json:"timeout,omitempty"` // seconds, zero selects 120
	Quiet     int               `json:"quiet,omitempty"`   // milliseconds, zero selects 1500
	Args      []string          `json:"args,omitempty"`
	CtrlC     bool              `json:"ctrl_c,omitempty"`
	Op        string            `json:"op"`
	Tab       string            `json:"tab,omitempty"` // stable id (t3) or an exact, unique name
	Text      string            `json:"text,omitempty"`
	Lines     int               `json:"lines,omitempty"`
	Token     string            `json:"token,omitempty"`
}

// TabInfo is a value in the Host's immutable tab snapshot.
type TabInfo struct {
	Parent       string            `json:"parent,omitempty"`
	Role         string            `json:"role,omitempty"`
	FirstMessage string            `json:"first_message,omitempty"`
	Labels       map[string]string `json:"labels"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Space        string            `json:"space"`
	Folder       string            `json:"folder"`
	Agent        string            `json:"agent"`
	Status       string            `json:"status"`
	Since        time.Time         `json:"since"`
	Workspace    bool              `json:"workspace"` // a workspace-run member
}

// Reply is the result of one operation. Error is present only on failure.
type Reply struct {
	Status    string    `json:"status,omitempty"`
	Warning   string    `json:"warning,omitempty"`
	OK        bool      `json:"ok"`
	Truncated bool      `json:"truncated,omitempty"` // transport shortened the result to its wire cap
	Tabs      []TabInfo `json:"tabs,omitempty"`
	Text      string    `json:"text,omitempty"`
	Error     *Error    `json:"error,omitempty"`
}

// Error carries a stable wire code and a human-readable explanation.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

const (
	CodeSpawnDisabled  = "spawn_disabled"
	CodeLimit          = orchestrate.CodeLimit
	CodeRateLimited    = orchestrate.CodeRateLimited
	CodeBadCWD         = orchestrate.CodeBadCWD
	CodeUnknownAgent   = orchestrate.CodeUnknownAgent
	CodeSpawnFailed    = orchestrate.CodeSpawnFailed
	CodeNotAllowed     = "not_allowed"
	CodeBusy           = "busy"
	CodeDisabled       = "disabled"
	CodeUnknownKey     = "unknown_key"
	CodeUnknownTab     = "unknown_tab"
	CodeAmbiguousTab   = "ambiguous_tab"
	CodeInvalidRequest = "invalid_request"
	CodeUnknownOp      = "unknown_op"
	CodeTextTooLarge   = orchestrate.CodeTextTooLarge
	CodeHostError      = "host_error"
)

// Host exposes thread-safe terminal operations without UI or transport types.
// Tabs returns an immutable snapshot; control never modifies it.
type Host interface {
	Tabs() []TabInfo
	// Keys encodes and submits each allowlisted key in order, stopping on failure.
	Keys(id string, keys []string) error
	// Interrupt sends Esc for a known agent, otherwise Ctrl+C; ctrlC forces Ctrl+C.
	Interrupt(id string, ctrlC bool) error
	// Screen returns the live visible screen plus up to lines recent scrollback
	// rows (0..MaxLines). On the alternate screen it returns only that screen.
	// The Host joins wrapped rows and trims trailing spaces. A session-backed
	// Host must read cells and wrap marks while holding the session lock.
	Screen(id string, lines int) (string, error)
	// Submit sends one text prompt. The Host handles paste encoding, a bounded
	// echo wait and Enter. The bytes are text, not pre-encoded terminal input.
	Submit(id string, text []byte) error
}
