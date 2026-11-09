package commands

import "testing"

func TestErrorLineLevel(t *testing.T) {
	cases := []struct {
		line string
		want string
		rank int
		ok   bool
	}{
		{"2026/10/24 10:00:00 [error] 12#0: *1 connect() failed", "error", 4, true},
		{"2026/10/24 10:00:00 [alert] 12#0: worker process 3 exited", "alert", 6, true},
		{"2026/10/24 10:00:00 [warn] 12#0: something", "warn", 3, true},
		{"2026/10/24 10:00:00 [notice] 12#0: signal 17", "notice", 2, true},
		{"no level here", "", 0, false},
		{"[bogus] x", "", 0, false},
	}
	for _, c := range cases {
		level, rank, ok := errorLineLevel(c.line)
		if level != c.want || rank != c.rank || ok != c.ok {
			t.Errorf("errorLineLevel(%q) = %q,%d,%v want %q,%d,%v", c.line, level, rank, ok, c.want, c.rank, c.ok)
		}
	}
}
