package domain

import (
	"testing"
	"time"
)

func TestDirectionOf(t *testing.T) {
	cases := map[string]Direction{
		"RSI (Daily Strong Bullish)":          Long,
		"RSI (GFS)":                           Long,
		"RSI (Bearish Advanced GFS)":          Short,
		"RSI (Weekly Strong Bearish)":         Short,
		"IchimokuKinkoHyo (SELL) 1A":          Short,
		"IchimokuKinkoHyo (SELL) Death Cross": Short,
		"IchimokuKinkoHyo (BUY) Gold Cross":   Long,
		"something short-dated":               Short,
	}
	for name, want := range cases {
		if got := DirectionOf(name); got != want {
			t.Errorf("DirectionOf(%q) = %v, want %v", name, got, want)
		}
	}
	if Long.String() != "long" || Short.String() != "short" {
		t.Errorf("String(): got %q/%q", Long.String(), Short.String())
	}
}

func TestTokenCurrent(t *testing.T) {
	utc := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	at1030 := utc("2026-09-24T05:00:00Z") // 10:30 IST
	cases := []struct {
		name    string
		created time.Time
		now     time.Time
		want    bool
	}{
		{"created 06:30 IST today", utc("2026-09-24T01:00:00Z"), at1030, true},
		{"created 05:59 IST today", utc("2026-09-24T00:29:00Z"), at1030, false},
		{"yesterday's token is still current before 06:00", utc("2026-09-23T03:00:00Z"), utc("2026-09-23T22:00:00Z"), true},
		{"yesterday's token after 06:00", utc("2026-09-23T03:00:00Z"), at1030, false},
		{"zero time", time.Time{}, at1030, false},
	}
	for _, tc := range cases {
		if got := TokenCurrent(tc.created, tc.now); got != tc.want {
			t.Errorf("%s: TokenCurrent = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestTradeDateIsIST(t *testing.T) {
	late := time.Date(2026, 9, 24, 20, 0, 0, 0, time.UTC) // 01:30 IST on the 25th
	if got := TradeDate(late); got != "2026-09-25" {
		t.Errorf("TradeDate = %q, want 2026-09-25", got)
	}
}
