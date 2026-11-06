package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/beefysalad/kaze-terminal/internal/version"
)

// The exe details Windows shows (Properties, Task Manager, antivirus) come from
// winres.json through the .syso files; they must say the release version.
func TestWinresMatchesVersion(t *testing.T) {
	b, err := os.ReadFile("winres/winres.json")
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		Version map[string]map[string]struct {
			Fixed struct {
				File    string `json:"file_version"`
				Product string `json:"product_version"`
			} `json:"fixed"`
			Info map[string]map[string]string `json:"info"`
		} `json:"RT_VERSION"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	v, ok := w.Version["#1"]["0000"]
	if !ok {
		t.Fatal("winres.json has no RT_VERSION #1/0000")
	}
	want4 := fourPart(version.Number)
	if v.Fixed.File != want4 || v.Fixed.Product != want4 {
		t.Errorf("fixed versions %q, %q; want %q", v.Fixed.File, v.Fixed.Product, want4)
	}
	en := v.Info["0409"]
	for _, k := range []string{"FileVersion", "ProductVersion"} {
		if en[k] != version.Number {
			t.Errorf("%s = %q, want %q", k, en[k], version.Number)
		}
	}
	for k, want := range map[string]string{"ProductName": "Kaze", "FileDescription": "Kaze terminal", "CompanyName": "patrick-gogo"} {
		if en[k] != want {
			t.Errorf("%s = %q, want %q", k, en[k], want)
		}
	}
}

// fourPart pads "0.32" or "0.13.1" to the four numbers Windows versions have.
func fourPart(v string) string {
	n := 1
	for _, c := range v {
		if c == '.' {
			n++
		}
	}
	for ; n < 4; n++ {
		v += ".0"
	}
	return v
}
