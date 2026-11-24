package control

import "testing"

func TestResolve(t *testing.T) {
	tabs := []TabInfo{
		{ID: "t1", Name: "review"},
		{ID: "t3", Name: "build"},
		{ID: "t4", Name: "review"},
		{ID: "t5", Name: "t3"},
		{ID: "t6"},
	}
	for _, tc := range []struct{ address, id, code string }{
		{"t1", "t1", ""},
		{"t3", "t3", ""}, // ids win over a colliding name
		{"build", "t3", ""},
		{"review", "", CodeAmbiguousTab},
		{"missing", "", CodeUnknownTab},
		{"Build", "", CodeUnknownTab},
		{" build ", "", CodeUnknownTab},
		{"", "", CodeInvalidRequest},
	} {
		t.Run(tc.address, func(t *testing.T) {
			got, err := Resolve(tabs, tc.address)
			if tc.code != "" {
				if err == nil || err.Code != tc.code || got.ID != "" {
					t.Fatalf("Resolve = %#v, %v; want %s", got, err, tc.code)
				}
				return
			}
			if err != nil || got.ID != tc.id {
				t.Fatalf("Resolve = %#v, %v; want %s", got, err, tc.id)
			}
		})
	}
	if _, err := Resolve(nil, "t1"); err == nil || err.Code != CodeUnknownTab {
		t.Fatalf("empty snapshot: %v", err)
	}
}
