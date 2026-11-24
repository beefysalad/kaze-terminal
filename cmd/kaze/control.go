package main

import (
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

// runControl performs one request without starting any window code.
func runControl(args []string, env func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	req, asJSON, raw, ok := controlArgsWithRaw(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: kaze ls [--json] | read <t> [--lines N] | send <t> <text|-> [--raw] [--if-idle] | name <t> <name> | label <t> k=v | keys <t> <key...> | interrupt <t> [--ctrl-c] | wait <t> [--for TEXT] [--timeout S] [--quiet MS]")
		if len(args) > 0 && args[0] == "keys" {
			fmt.Fprintln(stderr, "allowed keys:", control.AllowedKeys)
		}
		return 2
	}
	if req.Op == "send" && req.Text == "-" {
		if stdin == nil {
			fmt.Fprintln(stderr, "kaze: send - needs stdin")
			return 2
		}
		b, err := io.ReadAll(io.LimitReader(stdin, control.MaxTextBytes+1))
		if err != nil {
			fmt.Fprintln(stderr, "kaze: stdin:", err)
			return 1
		}
		req.Text = string(b)
	}
	if len(req.Text) > control.MaxTextBytes {
		fmt.Fprintln(stderr, "kaze: send text exceeds 32 KiB")
		return 2
	}
	if req.Op == "send" && !raw {
		req.From = env("KAZE_TAB_ID")
		if req.From != "" && !strings.HasPrefix(req.From, "t") {
			req.From = "t" + req.From
		}
	}
	addr := env("KAZE_SOCKET")
	if addr == "" {
		addr = env("KAZE_NOTIFY")
	}
	req.Token = env("KAZE_NOTIFY_TOKEN")
	if addr == "" {
		fmt.Fprintln(stderr, "kaze: KAZE_SOCKET (or KAZE_NOTIFY) is required; run inside a Kaze tab")
		return 1
	}
	if req.Token == "" {
		fmt.Fprintln(stderr, "kaze: KAZE_NOTIFY_TOKEN is required; run inside a Kaze tab")
		return 1
	}
	conn, err := net.DialTimeout("tcp", addr, controlTimeout)
	if err != nil {
		return controlNetworkError(err, stderr)
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(controlTimeout)); err != nil {
		return controlNetworkError(err, stderr)
	}
	if err = json.NewEncoder(conn).Encode(req); err != nil {
		return controlNetworkError(err, stderr)
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
		case "refused", "busy_server", control.CodeBusy, control.CodeDisabled:
			return 3
		case "timeout":
			return 124
		}
		return 1
	}
	if !reply.OK {
		fmt.Fprintln(stderr, "kaze: invalid_reply: reply is missing ok or error")
		return 1
	}
	if reply.Warning != "" {
		fmt.Fprintln(stderr, "kaze: warning:", reply.Warning)
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
				if _, err = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", tab.ID, name, tab.Space, agent, tab.Status, tab.Folder, controlLabels(tab.Labels)); err != nil {
					break
				}
			}
		case "read":
			_, err = io.WriteString(stdout, reply.Text)
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
		ok = len(args) == 1 || (len(args) == 2 && args[1] == "--json")
		asJSON = len(args) == 2
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
		case "ls", "read", "send", "keys", "interrupt", "wait", "name", "label":
			return true, runControl(args, env, stdin, stdout, stderr)
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
