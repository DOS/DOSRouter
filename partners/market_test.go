package partners

import "testing"

func TestSplitPair(t *testing.T) {
	cases := []struct{ in, base, quote string }{
		{"BTC-USD", "BTC", "USD"},
		{"eur-usd", "EUR", "USD"},
		{"ETH/USD", "ETH", "USD"},
		{"XAU-USD", "XAU", "USD"},
		{"BTCUSD", "", ""}, // no separator
		{"", "", ""},
	}
	for _, c := range cases {
		b, q := splitPair(c.in)
		if b != c.base || q != c.quote {
			t.Errorf("splitPair(%q) = (%q,%q), want (%q,%q)", c.in, b, q, c.base, c.quote)
		}
	}
}

func TestStripMarketSuffix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"AAPL", "AAPL"},
		{"0700-HK", "0700"},
		{"TLRY-CA", "TLRY"},
		{"BRK-A", "BRK-A"},  // single-letter suffix preserved
		{"AAPL-USD", "AAPL-USD"}, // 3-letter suffix preserved
		{"", ""},
	}
	for _, c := range cases {
		got := stripMarketSuffix(c.in)
		if got != c.want {
			t.Errorf("stripMarketSuffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveSymbolPrefersCanonical(t *testing.T) {
	feeds := []PythFeed{
		{ID: "1", Attributes: map[string]string{"base": "AAPL", "country": "US", "symbol": "Equity.US.AAPL/USD.PRE"}},
		{ID: "2", Attributes: map[string]string{"base": "AAPL", "country": "US", "symbol": "Equity.US.AAPL/USD.POST"}},
		{ID: "3", Attributes: map[string]string{"base": "AAPL", "country": "US", "symbol": "Equity.US.AAPL/USD"}},
		{ID: "4", Attributes: map[string]string{"base": "AAPL", "country": "US", "symbol": "Equity.US.AAPL/USD.ON"}},
	}
	got := ResolveSymbol(feeds, "AAPL", "us")
	if got == nil || got.ID != "3" {
		t.Errorf("expected canonical feed id=3, got %+v", got)
	}
}

func TestResolveSymbolFiltersCountry(t *testing.T) {
	feeds := []PythFeed{
		{ID: "1", Attributes: map[string]string{"base": "APPL", "country": "US", "symbol": "Equity.US.APPL/USD"}},
		{ID: "2", Attributes: map[string]string{"base": "APPL", "country": "HK", "symbol": "Equity.HK.APPL/HKD"}},
	}
	got := ResolveSymbol(feeds, "APPL", "hk")
	if got == nil || got.ID != "2" {
		t.Errorf("expected HK feed id=2, got %+v", got)
	}
}

func TestPow10(t *testing.T) {
	cases := []struct {
		e    int
		want float64
	}{
		{0, 1},
		{2, 100},
		{-2, 0.01},
		{-8, 1e-8},
	}
	for _, c := range cases {
		got := pow10(c.e)
		diff := got - c.want
		if diff < 0 {
			diff = -diff
		}
		// Floating-point divisions for negative exponents accumulate tiny
		// rounding error; allow a relative tolerance.
		tol := c.want * 1e-12
		if tol < 1e-14 {
			tol = 1e-14
		}
		if diff > tol {
			t.Errorf("pow10(%d) = %v, want %v (diff %v > tol %v)", c.e, got, c.want, diff, tol)
		}
	}
}
