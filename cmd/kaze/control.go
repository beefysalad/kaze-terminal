package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/beefysalad/kaze-terminal/internal/control"
)

const controlTimeout = 5 * time.Second

// runControl performs one request without starting any window code.
func runControl(args []string, env func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	req, asJSON, ok := controlArgs(args)
	if !ok {
		fmt.Fprintln(stderr, "usage: kaze ls [--json] | read <t> [--lines N] | send <t> <text|->")
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
		fmt.Fprintln(stderr, "kaze:", err)
		return 1
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(controlTimeout)); err != nil {
		fmt.Fprintln(stderr, "kaze:", err)
		return 1
	}
	if err = json.NewEncoder(conn).Encode(req); err != nil {
		fmt.Fprintln(stderr, "kaze:", err)
		return 1
	}
	var reply control.Reply
	if err = json.NewDecoder(io.LimitReader(conn, 1<<20)).Decode(&reply); err != nil {
		fmt.Fprintln(stderr, "kaze:", err)
		return 1
	}
	if reply.Error != nil {
		fmt.Fprintf(stderr, "kaze: %s: %s\n", reply.Error.Code, reply.Error.Message)
		switch reply.Error.Code {
		case "refused":
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
				if _, err = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\n", tab.ID, name, tab.Space, agent, tab.Status, tab.Folder); err != nil {
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

func controlArgs(args []string) (req control.Request, asJSON, ok bool) {
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
		ok = req.Tab != ""
	case "send":
		if len(args) != 3 {
			return
		}
		req.Tab, req.Text = args[1], args[2]
		ok = req.Tab != ""
	}
	return
}
