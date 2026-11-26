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

const controlUsage = "usage: kaze ls [--children] [--json] | read <t> [--lines N] | send <t> <text|-> [--raw] [--if-idle] | name <t> <name> | label <t> k=v | keys <t> <key...> | interrupt <t> [--ctrl-c] | wait --children all|any [--timeout S] | wait <t> [--for TEXT] [--timeout S] [--quiet MS] | run [--name N] [--cwd D] [--label k=v]... [--mode read-only|edit] [--wait-start] <claude|codex|template> <task|-> | kill <t> | kill --children | report <text|-> | templates\nrun flags may appear before or immediately after the agent/template, before the final task. Use - (stdin) or -- before a task starting with -.\nAddress tabs by id from ls. ls marks your tab self; tabs in another space belong to another project."

// runControl performs one request without starting any window code.
func runControl(args []string, env func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	req, asJSON, raw, ok := controlArgsWithRaw(args)
	if !ok {
		if len(args) > 0 && args[0] == "run" {
			_, err := runArgs(args[1:])
			fmt.Fprintln(stderr, "kaze: run:", err)
		}
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
	if (req.Op == "send" && !raw) || req.Op == "spawn" || req.Op == "kill" || req.Op == "report" || (req.Op == "wait" && req.Children != "") || (req.Op == "ls" && hasControlFlag(args, "--children")) {
		req.From = controlCaller(env)
		if req.From == "" && req.Op != "send" {
			fmt.Fprintln(stderr, "kaze: KAZE_TAB_ID is required; run inside a Kaze tab")
			return 1
		}
	}
	if req.Op == "read" || req.Op == "keys" || req.Op == "interrupt" || (req.Op == "wait" && req.Children == "") {
		req.From = controlCaller(env)
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
	if req.Op == "spawn" {
		if err = conn.SetReadDeadline(time.Now().Add(controlReadBudget(req))); err != nil {
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
		if req.Children != "" && reply.Reason == "timeout" {
			printChildrenWait(stdout, reply)
		}
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
				self := "-"
				if tab.ID == controlCaller(env) {
					self = "self"
				}
				if _, err = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", tab.ID, name, tab.Space, agent, tab.Status, tab.Folder, controlLabels(tab.Labels), controlColumn(tab.Parent), controlColumn(tab.Role), self); err != nil {
					break
				}
			}
		case "wait":
			if req.Children != "" {
				err = printChildrenWait(stdout, reply)
			}
		case "spawn", "kill", "templates":
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
				if tab.Space != "" {
					parts = append(parts, "space "+tab.Space)
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

// runArgs accepts options on either side of the agent, before the final task.
func runArgs(args []string) (req control.Request, err error) {
	req.Op = "spawn"
	seen := map[string]bool{}
	literal := false
	for i := 0; i < len(args); {
		flag := args[i]
		if !literal && flag == "--" {
			literal = true
			i++
			continue
		}
		if !literal && flag != "-" && strings.HasPrefix(flag, "-") {
			if seen[flag] && flag != "--label" {
				return req, fmt.Errorf("duplicate flag %s", flag)
			}
			seen[flag] = true
			if flag == "--wait-start" {
				req.WaitStart = true
				i++
				continue
			}
			switch flag {
			case "--name", "--cwd", "--label", "--mode":
			default:
				return req, fmt.Errorf("unknown flag %s", flag)
			}
			if i+1 >= len(args) || args[i+1] == "" {
				return req, fmt.Errorf("missing value for %s", flag)
			}
			value := args[i+1]
			switch flag {
			case "--name":
				name, e := control.ValidateName(nil, "", value)
				if e != nil {
					return req, fmt.Errorf("invalid value for %s", flag)
				}
				req.Name = name
			case "--cwd":
				req.CWD = value
			case "--mode":
				if value != "edit" && value != "read-only" {
					return req, fmt.Errorf("invalid value for %s", flag)
				}
				req.Mode = value
			case "--label":
				key, v, found := strings.Cut(value, "=")
				if !found || control.ValidateLabel(key, v) != nil {
					return req, fmt.Errorf("invalid value for %s", flag)
				}
				if req.Labels == nil {
					req.Labels = map[string]string{}
				}
				if _, exists := req.Labels[key]; exists || len(req.Labels) >= 8 {
					return req, fmt.Errorf("invalid value for %s", flag)
				}
				req.Labels[key] = v
			default:
				return req, fmt.Errorf("unknown flag %s", flag)
			}
			i += 2
			continue
		}
		if req.Agent == "" {
			if flag == "" || strings.HasPrefix(flag, "-") {
				return req, fmt.Errorf("expected an agent or template")
			}
			req.Agent = flag
			i++
			continue
		}
		if i != len(args)-1 {
			return req, fmt.Errorf("unexpected argument %q after task %q", args[i+1], flag)
		}
		if strings.TrimSpace(flag) == "" {
			return req, fmt.Errorf("task must not be empty")
		}
		req.Task = flag
		return req, nil
	}
	return req, fmt.Errorf("expected an agent or template and a final task")
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
		var err error
		req, err = runArgs(args[1:])
		ok = err == nil
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
		if len(args) > 1 && args[1] == "--children" {
			if len(args) != 3 && len(args) != 5 {
				return
			}
			req.Children = args[2]
			req.Timeout = control.DefaultWaitTimeout
			if req.Children != "all" && req.Children != "any" {
				return
			}
			if len(args) == 5 {
				if args[3] != "--timeout" {
					return
				}
				n, e := strconv.Atoi(args[4])
				if e != nil || n <= 0 || n > control.MaxWaitTimeout {
					return
				}
				req.Timeout = n
			}
			ok = true
			return
		}
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

func printChildrenWait(out io.Writer, r control.Reply) error {
	for _, tab := range r.Tabs {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\n", tab.ID, tab.Status, tab.Name); err != nil {
			return err
		}
	}
	if r.Text != "" {
		if _, err := fmt.Fprintln(out, r.Text); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, r.Reason)
	return err
}

// Consent may take sixty seconds before the normal start and delivery path.
// Keep a client margin beyond the corresponding server request budget.
func controlReadBudget(req control.Request) time.Duration {
	if req.Op == "spawn" {
		if req.WaitStart {
			return 100*time.Second + controlTimeout
		}
		return 65*time.Second + controlTimeout
	}
	return controlTimeout
}
