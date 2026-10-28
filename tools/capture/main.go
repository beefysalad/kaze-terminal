// Command capture runs a command in a pseudo-console and writes its raw output to a file.
// It never sends input; it kills the process after -d.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/patrick-gogo/kaze/internal/ptyx"
)

func main() {
	out := flag.String("o", "capture.bin", "output file")
	d := flag.Duration("d", 5*time.Second, "how long to record")
	flag.Parse()
	argv := flag.Args()
	if len(argv) == 0 {
		argv = ptyx.DefaultShell()
	}
	sess, err := ptyx.Start(argv, 120, 30, nil, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}
	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	go io.Copy(f, sess)
	time.Sleep(*d)
	sess.Close()
	time.Sleep(200 * time.Millisecond)
}
