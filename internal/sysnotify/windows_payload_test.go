package sysnotify

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestWindowsToastXMLIsSilentEscapedAndOpaque(t *testing.T) {
	n := Notification{ID: "a1-b2-c3", Title: `A < B & "C"`, Context: "branch > main", Body: "done\x00\x01\n日本語"}
	s := windowsToastXML(n)
	var got struct {
		Launch string `xml:"launch,attr"`
		Audio  struct {
			Silent string `xml:"silent,attr"`
		} `xml:"audio"`
		Texts []string `xml:"visual>binding>text"`
	}
	if err := xml.Unmarshal([]byte(s), &got); err != nil {
		t.Fatal(err, s)
	}
	if got.Launch != n.ID || got.Audio.Silent != "true" || len(got.Texts) != 3 || got.Texts[0] != n.Title || got.Texts[1] != n.Context || got.Texts[2] != "done\n日本語" {
		t.Fatalf("payload %#v: %s", got, s)
	}
	if strings.Contains(s, `activationType="protocol"`) {
		t.Fatal("must not execute a protocol")
	}
}

func TestWindowsToastIdentityIsShortStableAndValidated(t *testing.T) {
	for _, id := range []string{"1-2-3", "ffffffffffffffff-ffffffffffffffff-ffffffffffffffff"} {
		group, tag, err := windowsToastIdentity(id)
		if err != nil || len(group) > 16 || len(tag) > 16 {
			t.Fatalf("%q: %q %q %v", id, group, tag, err)
		}
		g2, t2, err := windowsToastIdentity(id[:strings.LastIndex(id, "-")] + "-a")
		if err != nil || g2 != group || t2 != tag {
			t.Fatal("revision changed replacement identity")
		}
	}
	for _, id := range []string{"", "one-two-three", "1-2", "1-2-3-4", "-2-3", "1-2-", "12345678901234567-1-1", "1/2-3-4", "1-2-3\x00"} {
		if _, _, err := windowsToastIdentity(id); err == nil {
			t.Errorf("accepted %q", id)
		}
	}
}

func TestWindowsShortcutOnlyRepairsCanonicalOwnedTarget(t *testing.T) {
	for _, exe := range []string{`C:\Users\A\Kaze\kaze.exe`, `c:\Users\A\Kaze\KAZE.EXE`} {
		if err := windowsShortcutTarget(exe, ""); err != nil {
			t.Fatal(err)
		}
		if err := windowsShortcutTarget(exe, `c:\users\a\kaze\kaze.exe`); err != nil {
			t.Fatal(err)
		}
	}
	for _, pair := range [][2]string{
		{`C:\Kaze\kaze-old.exe`, ""}, {`C:\Kaze\download.exe`, ""},
		{`C:\Kaze\kaze.exe`, `C:\Other\kaze.exe`}, {`C:\Kaze\kaze.exe`, `C:\Other\app.exe`},
		{"kaze.exe", ""}, {`C:\Kaze\kaze.exe`, `C:\Kaze\kaze-old.exe`},
	} {
		if err := windowsShortcutTarget(pair[0], pair[1]); err == nil {
			t.Errorf("accepted %q", pair)
		}
	}
	if windowsAUMID != "com.patrickgogo.kaze" {
		t.Fatal("stable bundle/Windows identity changed")
	}
}
