package battery

import "testing"

func TestFromPowerStatus(t *testing.T) {
	cases := []struct {
		name              string
		acLine, flag, pct byte
		want              Status
	}{
		{"no system battery", 1, 128, 255, Status{}},
		{"unknown status", 0, 255, 50, Status{}},
		{"unknown percent", 0, 1, 255, Status{}},
		{"82 on battery", 0, 1, 82, Status{Present: true, Percent: 82}},
		{"64 charging", 1, 8, 64, Status{Present: true, Charging: true, Percent: 64}},
		{"64 on AC without the flag", 1, 0, 64, Status{Present: true, Charging: true, Percent: 64}},
		{"100 on AC is not charging", 1, 1, 100, Status{Present: true, Percent: 100}},
		{"100 on AC with Windows' charging flag (flag 9, seen on a real laptop)", 1, 9, 100, Status{Present: true, Percent: 100}},
		{"18 low", 0, 2, 18, Status{Present: true, Percent: 18}},
		{"7 critical", 0, 4, 7, Status{Present: true, Percent: 7}},
		{"bogus 130 clamps", 0, 1, 130, Status{Present: true, Percent: 100}},
	}
	for _, c := range cases {
		if got := fromPowerStatus(c.acLine, c.flag, c.pct); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestFromIOPS(t *testing.T) {
	cases := []struct {
		name             string
		found, chg, onAC bool
		cur, max         int
		want             Status
	}{
		{"no internal battery", false, false, true, 0, 0, Status{}},
		{"zero max capacity", true, false, false, 50, 0, Status{}},
		{"71 on battery", true, false, false, 71, 100, Status{Present: true, Percent: 71}},
		{"64 charging", true, true, true, 64, 100, Status{Present: true, Charging: true, Percent: 64}},
		{"80 on AC, charging held", true, false, true, 80, 100, Status{Present: true, Charging: true, Percent: 80}},
		{"100 on AC is not charging", true, true, true, 100, 100, Status{Present: true, Percent: 100}},
		{"raw capacities round", true, false, false, 4245, 5103, Status{Present: true, Percent: 83}},
		{"over max clamps", true, false, false, 120, 100, Status{Present: true, Percent: 100}},
		{"negative clamps", true, false, false, -5, 100, Status{Present: true, Percent: 0}},
	}
	for _, c := range cases {
		if got := fromIOPS(c.found, c.chg, c.onAC, c.cur, c.max); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}
