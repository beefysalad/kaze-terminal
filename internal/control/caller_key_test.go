package control

import (
	"encoding/json"
	"testing"
)

type ownerKeyHost struct{ fakeHost }

func (h *ownerKeyHost) OwnerKey(id string) string {
	if id == "t1" {
		return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}
	if id == "t2" {
		return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	}
	return ""
}

func TestOwnerKeyGate(t *testing.T) {
	h := &ownerKeyHost{}
	for _, op := range []string{"spawn", "kill"} {
		for _, tc := range []struct {
			from, key string
			allowed   bool
		}{
			{"t1", "", false}, {"t1", "wrong", false},
			{"t1", h.OwnerKey("t1"), true},
			{"t2", h.OwnerKey("t1"), false},
			{"t3", "", false}, {"t3", h.OwnerKey("t1"), false},
			{"t99", h.OwnerKey("t1"), false}, {"", h.OwnerKey("t1"), false},
			{"lead", h.OwnerKey("t1"), false},
		} {
			raw, _ := json.Marshal(map[string]string{"op": op, "from": tc.from, "caller_key": tc.key})
			var req Request
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatal(err)
			}
			r := Handle(h, req)
			want := "not_allowed"
			if tc.allowed {
				want = CodeUnknownOp
			} // Operation is deliberately not implemented yet.
			if r.OK || r.Error == nil || r.Error.Code != want {
				t.Fatalf("%s from %q allowed=%v: %+v", op, tc.from, tc.allowed, r)
			}
			if !tc.allowed && r.Error.Message != "caller is not allowed" {
				t.Fatal("authentication failure leaked detail")
			}
		}
	}
	if r := Handle(&fakeHost{}, Request{Op: "spawn"}); r.Error == nil || r.Error.Code != "not_allowed" {
		t.Fatalf("host without verifier: %+v", r)
	}
}

func TestExistingOperationsNeedNoOwnerKey(t *testing.T) {
	for _, op := range []string{"ls", "read", "send", "keys", "interrupt", "wait", "name", "label"} {
		r := Handle(&fakeHost{}, Request{Op: op})
		if r.Error != nil && r.Error.Code == "not_allowed" {
			t.Fatalf("%s requires owner key", op)
		}
	}
}
