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
