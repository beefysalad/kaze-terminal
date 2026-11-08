package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func main() {
	in, _ := io.ReadAll(io.LimitReader(os.Stdin, 4096))
	cwd, _ := os.Getwd()
	_ = json.NewEncoder(os.Stdout).Encode(struct {
		Args                  []string
		Tab, Home, Cwd, Input string
	}{os.Args[1:], os.Getenv("KAZE_TAB_ID"), os.Getenv("CODEX_HOME"), cwd, string(in)})
	fmt.Fprint(os.Stderr, "fake error")
	os.Exit(23)
}
