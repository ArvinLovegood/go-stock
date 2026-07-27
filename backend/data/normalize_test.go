package data

import "testing"

func TestNormalizeStockCode(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"600938.SH", "sh600938"},
		{"600938", "sh600938"},
		{"sh600938", "sh600938"},
		{"SH600938", "sh600938"},
		{"000756.SZ", "sz000756"},
		{"sz000756", "sz000756"},
		{"usAAPL", "gb_aapl"},
		{"gb_AAPL", "gb_aapl"},
		{"hk00700", "hk00700"},
		{"00700.HK", "hk00700"},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeStockCode(c.in)
		if got != c.want {
			t.Errorf("normalizeStockCode(%q) = %q, want %q", c.in, got, c.want)
		} else {
			t.Logf("✓ normalizeStockCode(%q) = %q", c.in, got)
		}
	}
}
