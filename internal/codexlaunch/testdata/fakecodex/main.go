package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	if n := len(os.Args); n >= 3 && os.Args[n-2] == "--no-daemon" && os.Args[n-1] == "--help" {
		return // the launcher asks first whether the flag is supported
	}
	in, _ := io.ReadAll(io.LimitReader(os.Stdin, 4096))
	cwd, _ := os.Getwd()
	_ = json.NewEncoder(os.Stdout).Encode(struct {
		Args                  []string
		Tab, Home, Cwd, Input string
	}{os.Args[1:], os.Getenv("KAZE_TAB_ID"), os.Getenv("CODEX_HOME"), cwd, string(in)})
	fmt.Fprint(os.Stderr, "fake error")
	os.Exit(23)
}
