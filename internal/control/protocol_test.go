package control

import (
	"encoding/json"
	"testing"
)

func TestProtocolJSON(t *testing.T) {
	var req Request
	if err := json.Unmarshal([]byte(`{"op":"send","tab":"review","text":"hello\nworld","lines":12,"token":"test-token"}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.Op != "send" || req.Tab != "review" || req.Text != "hello\nworld" || req.Lines != 12 || req.Token != "test-token" {
		t.Fatalf("request = %#v", req)
	}
	data, err := json.Marshal(Reply{OK: false, Error: &Error{Code: CodeUnknownTab, Message: "unknown tab"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"ok":false,"error":{"code":"unknown_tab","message":"unknown tab"}}` {
		t.Fatalf("reply = %s", data)
	}
}

func TestReplyTruncationFlag(t *testing.T) {
	var reply Reply
	if err := json.Unmarshal([]byte(`{"ok":true,"text":"prefix","truncated":true}`), &reply); err != nil {
		t.Fatal(err)
	}
	if !reply.Truncated {
		t.Fatal("transport truncation flag lost")
	}
	b, err := json.Marshal(reply)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["truncated"] != true {
		t.Fatalf("flag missing: %s", b)
	}
	b, err = json.Marshal(Reply{OK: true})
	if err != nil {
		t.Fatal(err)
	}
	// Decode into a fresh map to check that ordinary replies omit the flag.
	wire = nil
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	if _, exists := wire["truncated"]; exists {
		t.Fatalf("unexpected flag: %s", b)
	}
}
