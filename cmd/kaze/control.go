package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/control"
)

const controlTimeout = 5 * time.Second
const controlMaxRequest = 64 << 10 // matches the notify server's JSON object cap

const controlUsage = "usage: kaze ls [--children] [--json] | read <t> [--lines N] | send <t> <text|-> [--raw] [--if-idle] | name <t> <name> | label <t> k=v | keys <t> <key...> | interrupt <t> [--ctrl-c] | wait <t> [--for TEXT] [--timeout S] [--quiet MS] | run [--name N] [--cwd D] [--label k=v]... [--mode read-only|edit] [--wait-start] <claude|codex|template> <task|-> | kill <t> | kill --children | report <text|-> | templates"

// runControl performs one request without starting any window code.
func runControl(args []string, env func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	req, asJSON, raw, ok := controlArgsWithRaw(args)
	if !ok {
		if len(args) == 3 && args[0] == "name" {
			if _, e := control.ValidateName(nil, "", args[2]); e != nil {
				fmt.Fprintln(stderr, "kaze: name:", e.Message)
				return 2
			}
		}
		fmt.Fprintln(stderr, controlUsage)
		if len(args) > 0 && args[0] == "keys" {
			fmt.Fprintln(stderr, "allowed keys:", control.AllowedKeys)
		}
		return 2
	}
	if req.Op == "templates" {
		if _, err := fmt.Fprintln(stdout, "no templates"); err != nil {
			return 1
		}
		return 0
	}
	if req.Op == "kill" && req.Tab == "--children" {
		return killControlChildren(env, stdout, stderr)
	}
	text, capBytes := &req.Text, control.MaxTextBytes
	switch req.Op {
	case "spawn":
		text, capBytes = &req.Task, 16<<10
	case "report":
		capBytes = control.MaxReportBytes
	}
	if (req.Op == "send" || req.Op == "spawn" || req.Op == "report") && *text == "-" {
		if stdin == nil {
			fmt.Fprintln(stderr, "kaze: - needs stdin")
			return 2
		}
		b, err := io.ReadAll(io.LimitReader(stdin, int64(capBytes+1)))
		if err != nil {
			fmt.Fprintln(stderr, "kaze: stdin:", err)
			return 1
		}
		*text = string(b)
	}
	if len(*text) > capBytes {
		fmt.Fprintf(stderr, "kaze: text_too_large: text exceeds %d KiB\n", capBytes>>10)
		return 2
	}
	if (req.Op == "send" && !raw) || req.Op == "spawn" || req.Op == "kill" || req.Op == "report" || (req.Op == "ls" && hasControlFlag(args, "--children")) {
		req.From = controlCaller(env)
		if req.From == "" && req.Op != "send" {
			fmt.Fprintln(stderr, "kaze: KAZE_TAB_ID is required; run inside a Kaze tab")
			return 1
		}
	}
	addr := env("KAZE_SOCKET")
	if addr == "" {
		addr = env("KAZE_NOTIFY")
	}
	req.Token = env("KAZE_NOTIFY_TOKEN")
	req.CallerKey = env("KAZE_TAB_KEY")
	if addr == "" {
		fmt.Fprintln(stderr, "kaze: KAZE_SOCKET (or KAZE_NOTIFY) is required; run inside a Kaze tab")
		return 1
	}
	if req.Token == "" {
		fmt.Fprintln(stderr, "kaze: KAZE_NOTIFY_TOKEN is required; run inside a Kaze tab")
		return 1
	}
	var wire bytes.Buffer
	encoder := json.NewEncoder(&wire)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(req); err != nil {
		fmt.Fprintln(stderr, "kaze: encode request:", err)
		return 2
	}
	// The server decodes one complete JSON object without needing a newline.
	encoded := bytes.TrimSuffix(wire.Bytes(), []byte{'\n'})
	if len(encoded) > controlMaxRequest {
		fmt.Fprintln(stderr, "kaze: encoded request exceeds 64 KiB; shorten the text or arguments")
		return 2
	}
	conn, err := net.DialTimeout("tcp", addr, controlTimeout)
	if err != nil {
		return controlNetworkError(err, stderr)
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(controlTimeout)); err != nil {
		return controlNetworkError(err, stderr)
	}
	if _, err = conn.Write(encoded); err != nil {
		return controlNetworkError(err, stderr)
	}
	if req.Op == "spawn" && req.WaitStart {
		// The server allows five seconds to apply plus thirty for delivery.
		if err = conn.SetReadDeadline(time.Now().Add(35*time.Second + controlTimeout)); err != nil {
			return controlNetworkError(err, stderr)
		}
	}
	if req.Op == "wait" {
		if err = conn.SetReadDeadline(time.Now().Add(time.Duration(req.Timeout)*time.Second + controlTimeout)); err != nil {
			return controlNetworkError(err, stderr)
		}
	}
	var reply control.Reply
	if err = json.NewDecoder(io.LimitReader(conn, 1<<20)).Decode(&reply); err != nil {
		return controlNetworkError(err, stderr)
	}
	if reply.Error != nil {
		fmt.Fprintf(stderr, "kaze: %s: %s\n", reply.Error.Code, reply.Error.Message)
		switch reply.Error.Code {
		case control.CodeUnknownKey:
			fmt.Fprintln(stderr, "allowed keys:", control.AllowedKeys)
			return 2
		case "refused", "busy_server", control.CodeBusy, control.CodeDisabled, control.CodeNotAllowed, control.CodeSpawnDisabled, control.CodeLimit, control.CodeRateLimited, "consent_denied":
			return 3
		case control.CodeBadCWD, control.CodeUnknownAgent, "unknown_template", control.CodeTextTooLarge:
			return 2
		case "timeout", "consent_timeout":
			return 124
		}
		return 1
	}
	if !reply.OK {
		fmt.Fprintln(stderr, "kaze: invalid_reply: reply is missing ok or error")
		return 1
	}
	if req.Op == "read" && reply.Truncated {
		fmt.Fprintln(stderr, "kaze: warning: read truncated to the newest complete lines that fit the 1 MiB reply cap")
	}
	if reply.Warning != "" {
		fmt.Fprintln(stderr, "kaze: warning:", reply.Warning)
	}
	if req.Op == "ls" && hasControlFlag(args, "--children") {
		children := make([]control.TabInfo, 0, len(reply.Tabs))
		for _, tab := range reply.Tabs {
			if tab.Parent == req.From && tab.Role == "worker" {
				children = append(children, tab)
			}
		}
		reply.Tabs = children
	}
	if asJSON {
		err = json.NewEncoder(stdout).Encode(reply)
	} else {
		switch req.Op {
		case "ls":
			for _, tab := range reply.Tabs {
				name, agent := tab.Name, tab.Agent
				if name == "" {
					name = "-"
				}
				if agent == "" {
					agent = "-"
				}
				if _, err = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", tab.ID, name, tab.Space, agent, tab.Status, tab.Folder, controlLabels(tab.Labels), controlColumn(tab.Parent), controlColumn(tab.Role)); err != nil {
					break
				}
			}
		case "spawn", "kill":
			_, err = fmt.Fprintln(stdout, reply.Text)
		case "read":
			_, err = io.WriteString(stdout, reply.Text)
		case "send":
			// Say where the text went: silence made agents think it was lost.
			if len(reply.Tabs) == 1 {
				tab := reply.Tabs[0]
				var parts []string
				for _, p := range []string{tab.Agent, tab.Name, tab.Status} {
					if p != "" {
						parts = append(parts, p)
					}
				}
				_, err = fmt.Fprintf(stdout, "sent to %s (%s)\n", tab.ID, strings.Join(parts, ", "))
			}
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "kaze: output:", err)
		return 1
	}
	return 0
}

func controlArgs(args []string) (control.Request, bool, bool) {
	req, asJSON, _, ok := controlArgsWithRaw(args)
	return req, asJSON, ok
}
func controlArgsWithRaw(args []string) (req control.Request, asJSON, raw, ok bool) {
	if len(args) == 0 {
		return
	}
	req.Op = args[0]
	switch req.Op {
	case "ls":
		seen := map[string]bool{}
		for _, flag := range args[1:] {
			if (flag != "--json" && flag != "--children") || seen[flag] {
				return
			}
			seen[flag] = true
		}
		asJSON, ok = seen["--json"], true
	case "run":
		req.Op = "spawn"
		seen := map[string]bool{}
		i := 1
		for i < len(args) && strings.HasPrefix(args[i], "--") {
			flag := args[i]
			if seen[flag] && flag != "--label" {
				return
			}
			seen[flag] = true
			if flag == "--wait-start" {
				req.WaitStart = true
				i++
				continue
			}
			if i+1 >= len(args) {
				return
			}
			value := args[i+1]
			if value == "" {
				return
			}
			switch flag {
			case "--name":
				name, e := control.ValidateName(nil, "", value)
				if e != nil {
					return
				}
				req.Name = name
			case "--cwd":
				req.CWD = value
			case "--mode":
				if value != "edit" && value != "read-only" {
					return
				}
				req.Mode = value
			case "--label":
				key, v, found := strings.Cut(value, "=")
				if !found || control.ValidateLabel(key, v) != nil {
					return
				}
				if req.Labels == nil {
					req.Labels = map[string]string{}
				}
				if _, exists := req.Labels[key]; exists || len(req.Labels) >= 8 {
					return
				}
				req.Labels[key] = v
			default:
				return
			}
			i += 2
		}
		if len(args)-i != 2 {
			return
		}
		req.Agent, req.Task = args[i], args[i+1]
		ok = req.Agent != "" && !strings.HasPrefix(req.Agent, "-") && strings.TrimSpace(req.Task) != ""
	case "kill":
		if len(args) != 2 {
			return
		}
		req.Tab = args[1]
		ok = req.Tab == "--children" || (req.Tab != "" && !strings.HasPrefix(req.Tab, "-"))
	case "report":
		if len(args) != 2 {
			return
		}
		req.Text = args[1]
		ok = strings.TrimSpace(req.Text) != "" && !strings.HasPrefix(req.Text, "--")
	case "templates":
		ok = len(args) == 1
	case "read":
		if len(args) != 2 && len(args) != 4 {
			return
		}
		req.Tab = args[1]
		if len(args) == 4 {
			if args[2] != "--lines" {
				return
			}
			n, err := strconv.Atoi(args[3])
			if err != nil || n < 0 || n > control.MaxLines {
				return
			}
			req.Lines = n
		}
		ok = req.Tab != "" && !strings.HasPrefix(req.Tab, "-")
	case "keys":
		if len(args) < 3 || len(args)-2 > control.MaxKeys {
			return
		}
		req.Tab, req.Args = args[1], args[2:]
		for _, name := range req.Args {
			if !control.ValidKey(name) {
				return
			}
		}
		ok = req.Tab != "" && !strings.HasPrefix(req.Tab, "-")
	case "wait":
		if len(args) < 2 || len(args)%2 != 0 {
			return
		}
		req.Tab = args[1]
		req.Timeout = control.DefaultWaitTimeout
		req.Quiet = control.DefaultQuiet
		seen := map[string]bool{}
		for i := 2; i < len(args); i += 2 {
			flag, value := args[i], args[i+1]
			if seen[flag] {
				return
			}
			seen[flag] = true
			if flag == "--for" {
				if value == "" || len(value) > control.MaxTextBytes {
					return
				}
				req.For = value
				continue
			}
			n, err := strconv.Atoi(value)
			if err != nil || n <= 0 {
				return
			}
			switch flag {
			case "--timeout":
				if n > control.MaxWaitTimeout {
					return
				}
				req.Timeout = n
			case "--quiet":
				if n > int((1<<63-1)/int64(time.Millisecond)) {
					return
				}
				req.Quiet = n
			default:
				return
			}
		}
		ok = req.Tab != "" && !strings.HasPrefix(req.Tab, "-")
	case "interrupt":
		if len(args) != 2 && len(args) != 3 {
			return
		}
		if len(args) == 3 {
			if args[2] != "--ctrl-c" {
				return
			}
			req.CtrlC = true
		}
		req.Tab = args[1]
		ok = req.Tab != "" && !strings.HasPrefix(req.Tab, "-")
	case "name", "label":
		if len(args) != 3 {
			return
		}
		req.Tab = args[1]
		if req.Op == "name" {
			name, e := control.ValidateName(nil, "", args[2])
			if e != nil {
				return
			}
			req.Name = name
		} else {
			key, value, found := strings.Cut(args[2], "=")
			if !found || control.ValidateLabel(key, value) != nil {
				return
			}
			req.Key, req.Value = key, value
		}
		ok = req.Tab != "" && !strings.HasPrefix(req.Tab, "-")
	case "send":
		positional := make([]string, 0, 2)
		flags := true
		for _, arg := range args[1:] {
			if flags && arg == "--" {
				flags = false
				continue
			}
			if flags && strings.HasPrefix(arg, "--") {
				switch arg {
				case "--raw":
					if raw {
						return
					}
					raw = true
				case "--if-idle":
					if req.IfIdle {
						return
					}
					req.IfIdle = true
				default:
					return
				}
			} else {
				positional = append(positional, arg)
			}
		}
		if len(positional) != 2 {
			return
		}
		req.Tab, req.Text = positional[0], positional[1]
		ok = req.Tab != "" && !strings.HasPrefix(req.Tab, "-")
	}
	return
}

// Transport timeouts are distinct from immediate failures such as connection refusal.
func controlNetworkError(err error, stderr io.Writer) int {
	fmt.Fprintln(stderr, "kaze:", err)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return 124
	}
	return 1
}

// dispatchControl leaves flags and arguments to the existing application paths.
func dispatchControl(args []string, env func(string) string, stdin io.Reader, stdout, stderr io.Writer) (bool, int) {
	if len(args) > 0 {
		switch args[0] {
		case "ls", "read", "send", "keys", "interrupt", "wait", "name", "label", "run", "kill", "report", "templates":
			return true, runControl(args, env, stdin, stdout, stderr)
		case "help", "--help", "-h":
			fmt.Fprintln(stdout, controlUsage)
			return true, 0
		}
		// A bare word is a command nobody implemented (an agent guessing, "list"
		// or "status"): say so, instead of starting a second Kaze window.
		// notify and statusline are Kaze's own commands, handled before this point.
		if !strings.HasPrefix(args[0], "-") && args[0] != "notify" && args[0] != "statusline" {
			fmt.Fprintf(stderr, "kaze: unknown command %q\n%s\n", args[0], controlUsage)
			return true, 2
		}
	}
	return false, 0
}

// controlLabels sorts keys so plain listings remain stable for agents.
func controlLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+labels[k])
	}
	return strings.Join(pairs, ",")
}

func controlColumn(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
func hasControlFlag(args []string, flag string) bool {
	for _, arg := range args[1:] {
		if arg == flag {
			return true
		}
	}
	return false
}
func controlCaller(env func(string) string) string {
	id := env("KAZE_TAB_ID")
	if id != "" && !strings.HasPrefix(id, "t") {
		id = "t" + id
	}
	return id
}

// Use the same bounded transport and exit handling for each request. A failed
// close ends the operation immediately, preserving output for prior successes.
func killControlChildren(env func(string) string, stdout, stderr io.Writer) int {
	var listing bytes.Buffer
	if code := runControl([]string{"ls", "--children", "--json"}, env, nil, &listing, stderr); code != 0 {
		return code
	}
	var reply control.Reply
	if err := json.Unmarshal(listing.Bytes(), &reply); err != nil {
		return 1
	}
	for _, child := range reply.Tabs {
		if code := runControl([]string{"kill", child.ID}, env, nil, stdout, stderr); code != 0 {
			return code
		}
	}
	return 0
}
