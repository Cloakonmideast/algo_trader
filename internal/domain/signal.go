package domain

import (
	"strings"
	"time"
)

// Direction is the side a signal takes. Its String() form ("long"/"short") is
// what the website and the Supabase signals table store.
type Direction int8

const (
	Long Direction = iota
	Short
)

func (d Direction) String() string {
	if d == Short {
		return "short"
	}
	return "long"
}

// shortMarkers is the bearish vocabulary strategies use in signal names.
// The website's directionOf (web/src/lib/catalog.ts) uses the same list.
var shortMarkers = []string{"SELL", "BEARISH", "DEATH", "SHORT"}

// DirectionOf classifies a signal name as Short when it carries bearish
// vocabulary, and Long otherwise.
func DirectionOf(signalName string) Direction {
	u := strings.ToUpper(signalName)
	for _, m := range shortMarkers {
		if strings.Contains(u, m) {
			return Short
		}
	}
	return Long
}

// RSIReadings are the RSI values behind an RSI signal. Weekly and monthly RSI
// need months of history, so each carries a flag saying whether it is known.
type RSIReadings struct {
	Daily     float64
	Weekly    float64
	WeeklyOK  bool
	Monthly   float64
	MonthlyOK bool
}

// IST is Asia/Kolkata, with a fixed +05:30 fallback when tzdata is missing.
var IST = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+30*60)
	}
	return loc
}()

// TradeDate is the IST calendar date of t, as YYYY-MM-DD.
func TradeDate(t time.Time) string {
	return t.In(IST).Format("2006-01-02")
}

// TokenCurrent reports whether a Kite access token created at createdAt is
// still usable at now. Zerodha expires tokens early each morning, so a token is
// current when it was created at or after the most recent 06:00 IST.
// The website applies the same rule (tokenValidToday in web/src/lib/time.ts).
func TokenCurrent(createdAt, now time.Time) bool {
	if createdAt.IsZero() {
		return false
	}
	n := now.In(IST)
	cutoff := time.Date(n.Year(), n.Month(), n.Day(), 6, 0, 0, 0, IST)
	if n.Before(cutoff) {
		cutoff = cutoff.AddDate(0, 0, -1)
	}
	return !createdAt.Before(cutoff)
}
