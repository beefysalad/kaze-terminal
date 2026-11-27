package fonts

import (
	"gioui.org/font/opentype"
	"os"
	"strings"
	"testing"
)

func TestBundledFontsParse(t *testing.T) {
	for _, b := range [][]byte{MonoRegular, MonoBold, SansRegular, SansSemiBold, SansBold, CJK} {
		if len(b) < 10000 {
			t.Fatal("short font")
		}
		if _, e := opentype.Parse(b); e != nil {
			t.Fatal(e)
		}
	}
}
func TestGeistLicenseShipped(t *testing.T) {
	b, e := os.ReadFile("OFL-Geist.txt")
	if e != nil || !strings.Contains(strings.ToLower(string(b)), "sil open font license") {
		t.Fatal(e)
	}
}
