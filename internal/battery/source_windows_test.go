package battery

import "testing"

func TestNativeReadOnWindows(t *testing.T) {
	st, err := Native().Read()
	if err != nil {
		t.Fatal(err)
	}
	if st.Percent < 0 || st.Percent > 100 {
		t.Fatalf("percent %d", st.Percent)
	}
	if !st.Present && (st.Percent != 0 || st.Charging) {
		t.Fatalf("no battery but %+v", st)
	}
	t.Logf("this machine: %+v", st)
}

func BenchmarkRead(b *testing.B) {
	src := Native()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := src.Read(); err != nil {
			b.Fatal(err)
		}
	}
}
