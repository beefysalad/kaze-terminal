// Command flood writes terminal output for kaze's benchmarks: N lines of mixed
// ASCII, Japanese and SGR colors as fast as it can, or (-claude) a captured
// Claude Code screen followed by a streamed response drawn the way Claude Code
// draws it. Sizes and run time are capped so it can never fill memory or run forever.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	maxLines   = 1_000_000
	maxSeconds = 300
	maxCapture = 16 << 20
)

func main() {
	lines := flag.Int("lines", 10000, "lines to print (max 1000000)")
	claude := flag.String("claude", "", "replay this raw capture (from tools/capture) instead of printing lines")
	seconds := flag.Int("seconds", 10, "how long to replay the capture (max 300)")
	every := flag.Duration("every", 16*time.Millisecond, "time between streamed repaints")
	flag.Parse()
	var err error
	if *claude != "" {
		err = replay(os.Stdout, *claude, min(max(*seconds, 1), maxSeconds), max(*every, time.Millisecond))
	} else {
		err = flood(os.Stdout, min(max(*lines, 0), maxLines))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "flood:", err)
		os.Exit(1)
	}
}

// line i: a numbered line with 16-color, 256-color and truecolor runs, bold, and Japanese text.
func line(b []byte, i int) []byte {
	b = fmt.Appendf(b, "\x1b[3%dm%06d\x1b[0m ", 1+i%7, i)
	b = fmt.Appendf(b, "\x1b[38;5;%dmthe quick brown fox\x1b[0m ", 16+i%216)
	b = fmt.Appendf(b, "\x1b[1;38;2;%d;%d;%dm日本語のテキスト\x1b[0m ", 80+i%170, 200-i%120, 120+i%130)
	b = fmt.Appendf(b, "\x1b[4%dm jumps \x1b[0m over the lazy dog ｶﾅ 漢字 %d\r\n", i%8, i*7919%100000)
	return b
}

func flood(w io.Writer, n int) error {
	start := time.Now()
	bw := bufio.NewWriterSize(w, 64<<10)
	var b []byte
	total := 0
	for i := 0; i < n; i++ {
		b = line(b[:0], i)
		total += len(b)
		if _, err := bw.Write(b); err != nil {
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(w, "flood: %d lines, %d bytes in %d ms\r\n", n, total, time.Since(start).Milliseconds())
	return nil
}

// strip drops the capture's queries and input-mode switches (their replies would
// land in the shell's input after the replay, and the modes would outlive it), and
// every OSC and DCS string: titles, OSC 52 clipboard writes and the like.
var strip = regexp.MustCompile(`\x1b\[\?(9001|1004|2004|2031)[hl]|\x1b\[[<>]\d*u|\x1b\[\?u|\x1b\[>4;\d+m|\x1b\[>0q|\x1b\][^\x07\x1b]*(?:\x07|\x1b\x5c)|\x1bP[^\x1b]*\x1b\x5c`)

// reset puts the terminal back the way a shell expects it.
const reset = "\x1b[?2026l\x1b[0m\x1b[?25h\r\n"

var words = strings.Fields(`The renderer keeps one strip per band of rows and only uploads a strip
when a row in it changed. ストリーミング中は 差分だけを 描画します。 Each frame copies the dirty rows
out under the lock, rasterizes them on the CPU, and hands Gio one image op per strip. 日本語の
テキストも 全角幅で 正しく 並びます。 That keeps a stream of tokens cheap: most frames touch two
or three rows, the spinner and the line being written.`)

// replay shows the captured Claude Code screen, then streams a response the
// way Claude Code's renderer does: finished lines are printed once and scroll
// into history, and a live region (the paragraph's last lines, a spinner and
// the input box) is redrawn in place each tick inside a synchronized update.
// The capture has no timestamps, so the pace is -every, not the original one.
func replay(w io.Writer, path string, seconds int, every time.Duration) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxCapture))
	f.Close()
	if err != nil {
		return err
	}
	frame := strip.ReplaceAll(raw, nil)
	if len(bytes.TrimSpace(frame)) == 0 {
		return fmt.Errorf("%s: nothing to replay", path)
	}
	start := time.Now()
	if _, err := w.Write(append(frame, "\x1b[0m\r\n\r\n"...)); err != nil {
		return err
	}
	const width, keep = 76, 4
	gray := "\x1b[38;2;136;136;136m"
	spin := []rune("✢✳✶✻✽")
	end := start.Add(time.Duration(seconds) * time.Second)
	t := time.NewTicker(every)
	defer t.Stop()
	var para []string // words of the paragraph being written
	var out bytes.Buffer
	n, total, wi, live := 0, len(frame), 0, 0
	for time.Now().Before(end) {
		<-t.C
		for range 3 {
			para = append(para, words[wi%len(words)])
			wi++
		}
		lines := wrap(para, width)
		out.Reset()
		out.WriteString("\x1b[?2026h")
		if live > 1 {
			fmt.Fprintf(&out, "\x1b[%dA", live-1)
		}
		out.WriteString("\r\x1b[J")
		if len(lines) > keep { // finished lines go above the live region, once
			for _, l := range lines[:len(lines)-keep] {
				out.WriteString(l + "\r\n")
			}
			lines = lines[len(lines)-keep:]
			para = strings.Fields(strings.Join(lines, " "))
		}
		reg := append(lines,
			"",
			fmt.Sprintf("\x1b[38;2;215;119;87m%c Streaming… \x1b[38;2;153;153;153m(%ds · ↓ %d tokens · esc to interrupt)\x1b[0m",
				spin[n%len(spin)], int(time.Since(start).Seconds()), wi),
			gray+"╭"+strings.Repeat("─", width-2)+"╮\x1b[0m",
			gray+"│\x1b[0m > "+strings.Repeat(" ", width-5)+gray+"│\x1b[0m",
			gray+"╰"+strings.Repeat("─", width-2)+"╯\x1b[0m")
		out.WriteString(strings.Join(reg, "\r\n"))
		out.WriteString("\x1b[?2026l")
		live = len(reg)
		if _, err := w.Write(out.Bytes()); err != nil {
			return err
		}
		n++
		total += out.Len()
	}
	fmt.Fprintf(w, "%sflood: capture plus %d streamed repaints, %d bytes in %d ms\r\n", reset, n, total, time.Since(start).Milliseconds())
	return nil
}

// wrap breaks words into lines of at most width cells, counting wide text as two cells a rune.
func wrap(ws []string, width int) []string {
	var lines []string
	var cur strings.Builder
	cw := 0
	for _, wd := range ws {
		n := cells(wd)
		if cw > 0 && cw+1+n > width {
			lines = append(lines, cur.String())
			cur.Reset()
			cw = 0
		}
		if cw > 0 {
			cur.WriteByte(' ')
			cw++
		}
		cur.WriteString(wd)
		cw += n
	}
	if cw > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

// cells is a rough display width: CJK and other wide runes count two.
func cells(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x1100 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
