package subagents

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
)

// maxPromptRead is all FirstPrompt reads of a transcript: a subagent's prompt
// is its first record, and a longer first line is not worth chasing (#135).
const maxPromptRead = 64 << 10

var promptBufs = sync.Pool{New: func() any { return new([maxPromptRead]byte) }}

// openPrompt opens a regular file for FirstPrompt; tests count what it reads.
var openPrompt = func(path string) (io.ReadCloser, error) {
	f, _, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// FirstPrompt is the prompt a subagent transcript starts with: the text of its
// first user record that is not only tool results. It reads at most
// maxPromptRead bytes once and only complete lines; "", false when the file is
// missing, holds no such record yet, or its first prompt is longer than that.
func FirstPrompt(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	f, err := openPrompt(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	buf := promptBufs.Get().(*[maxPromptRead]byte)
	defer promptBufs.Put(buf)
	n, err := io.ReadFull(f, buf[:])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", false
	}
	data := buf[:n]
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return "", false
		}
		if s, ok := userPrompt(data[:i]); ok {
			return s, true
		}
		data = data[i+1:]
	}
}

// userPrompt is a user record's message text: a JSON string, or its text
// blocks joined by newlines.
func userPrompt(line []byte) (string, bool) {
	var r struct {
		Type    string
		Message struct{ Content json.RawMessage }
	}
	if json.Unmarshal(line, &r) != nil || r.Type != "user" || len(r.Message.Content) == 0 {
		return "", false
	}
	var s string
	if json.Unmarshal(r.Message.Content, &s) == nil {
		return s, s != ""
	}
	var blocks []struct{ Type, Text string }
	if json.Unmarshal(r.Message.Content, &blocks) != nil {
		return "", false
	}
	var texts []string
	for _, b := range blocks {
		if b.Type == "text" {
			texts = append(texts, b.Text)
		}
	}
	s = strings.Join(texts, "\n")
	return s, s != ""
}
