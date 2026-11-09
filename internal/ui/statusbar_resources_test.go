package ui

import "testing"

// The resources group reads "CPU meter 3%  RAM 7.7 GB" when labelled, and
// falls back to "3% · 7.7 GB" once the labels are dropped to fit.
func TestResourcesGroupWidthCarriesRAMLabel(t *testing.T) {
	m := busyModel()
	d := statusTestDims
	cpu, ram := resourceValues(m.Resources.Total, m.Resources.Partial)
	labelled, bare := false, false
	for w := 2400; w >= 200; w-- {
		l := statusAt(m, w)
		r, ok := l.Groups[grpMem]
		if !ok || l.Tight {
			continue
		}
		want := 2*d.ItemPad + seven(cpu, true) + seven(ram, true)
		if l.Compact.ResourcesLabel {
			bare = true
			want += d.Gap + seven("·", false) + d.Gap
		} else {
			labelled = true
			want += seven("CPU", false) + d.Gap/2 + 2*d.Gap + seven("RAM", false) + d.Gap/2
		}
		if !l.Compact.ResourcesMeter {
			want += d.Meter + d.Gap/2
		}
		if r.Dx() != want {
			t.Fatalf("width %d (label dropped: %v): group is %d px, want %d", w, l.Compact.ResourcesLabel, r.Dx(), want)
		}
	}
	if !labelled || !bare {
		t.Fatalf("expected both forms across widths: labelled %v, bare %v", labelled, bare)
	}
}
