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
