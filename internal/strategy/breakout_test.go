package strategy

import (
	"math"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	"github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
)

// bar builds one daily candle dated at IST midnight, like Kite's.
func bar(y int, m time.Month, d int, open, high, low, close float64) kiteconnect.HistoricalData {
	return kiteconnect.HistoricalData{
		Date: models.Time{Time: time.Date(y, m, d, 0, 0, 0, 0, domain.IST)},
		Open: open, High: high, Low: low, Close: close,
	}
}

func istDay(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, domain.IST) }

// mayJune has May highs up to 110 and lows down to 90, then June up to (not
// including) Friday 2026-06-12, all inside that range.
func mayJune() []kiteconnect.HistoricalData {
	return []kiteconnect.HistoricalData{
		bar(2026, time.May, 26, 100, 105, 95, 100),
		bar(2026, time.May, 27, 100, 110, 92, 104), // May high 110
		bar(2026, time.May, 29, 100, 104, 90, 100), // May low 90
		bar(2026, time.June, 1, 100, 106, 94, 102),
		bar(2026, time.June, 11, 102, 108, 93, 105),
	}
}

func TestMonthlyBreak(t *testing.T) {
	june12 := istDay(2026, time.June, 12)
	cases := []struct {
		name        string
		before      []kiteconnect.HistoricalData
		today       time.Time
		open, price float64
		up, down    bool
	}{
		{"intraday break above May high", mayJune(), june12, 107, 112, true, false},
		{"gap open above May high", mayJune(), june12, 111, 109, true, false},
		{"still under May high", mayJune(), june12, 107, 109.5, false, false},
		{"intraday break below May low", mayJune(), june12, 95, 88, false, true},
		{"gap open below May low", mayJune(), june12, 89, 91, false, true},
		{"first trading day of the month", mayJune()[:3], istDay(2026, time.June, 1), 108, 111, true, false},
		{"already above May high earlier in June",
			append(mayJune(), bar(2026, time.June, 11, 105, 111, 104, 106)), june12, 107, 112, false, false},
		{"already below May low earlier in June",
			append(mayJune(), bar(2026, time.June, 11, 95, 96, 89, 94)), june12, 95, 88, false, false},
		{"January looks back to December",
			[]kiteconnect.HistoricalData{bar(2025, time.December, 31, 100, 110, 90, 100)},
			istDay(2026, time.January, 2), 105, 111, true, false},
		{"no candles for the previous month", mayJune()[3:], june12, 107, 200, false, false},
		{"no history at all", nil, june12, 107, 200, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := monthlyBreakUp(c.before, c.today, c.open, c.price); got != c.up {
				t.Errorf("monthlyBreakUp = %v, want %v", got, c.up)
			}
			if got := monthlyBreakDown(c.before, c.today, c.open, c.price); got != c.down {
				t.Errorf("monthlyBreakDown = %v, want %v", got, c.down)
			}
		})
	}
}

// weekdays returns n consecutive trading-day dates ending on end, skipping weekends.
func weekdays(end time.Time, n int) []time.Time {
	var out []time.Time
	for d := end; len(out) < n; d = d.AddDate(0, 0, -1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append([]time.Time{d}, out...)
		}
	}
	return out
}

func flat(dates []time.Time, price float64) []kiteconnect.HistoricalData {
	out := make([]kiteconnect.HistoricalData, len(dates))
	for i, d := range dates {
		out[i] = bar(d.Year(), d.Month(), d.Day(), price, price, price, price)
	}
	return out
}

func TestWeeklyConversionLine(t *testing.T) {
	today := istDay(2026, time.June, 12) // Friday
	dates := weekdays(today.AddDate(0, 0, -1), 60)
	before := flat(dates, 100)
	at := func(m time.Month, d int) *kiteconnect.HistoricalData {
		for i := range before {
			if before[i].Date.Time.Equal(istDay(2026, m, d)) {
				return &before[i]
			}
		}
		t.Fatalf("no candle on %v %d", m, d)
		return nil
	}

	// The window is the current week (from Mon 8 June) plus the 8 weeks before
	// it, so it starts on Mon 13 April.
	at(time.June, 11).High = 130 // Thu of the current week
	at(time.April, 13).Low = 70  // first day inside the window
	at(time.April, 10).High = 500
	at(time.April, 10).Low = 1 // Fri 10 April is outside

	got, ok := weeklyConversionLine(before, today, 101, 99)
	if !ok || math.Abs(got-100) > 1e-9 {
		t.Fatalf("weeklyConversionLine = %v, %v; want 100 ((130+70)/2), true", got, ok)
	}

	// Today's own range is part of the current week.
	if got, _ := weeklyConversionLine(before, today, 150, 99); math.Abs(got-110) > 1e-9 {
		t.Errorf("with today's high 150: got %v, want 110 ((150+70)/2)", got)
	}

	// Fewer than 9 weeks of history (4 days this week + 7 full weeks): not known.
	if _, ok := weeklyConversionLine(before[len(before)-39:], today, 101, 99); ok {
		t.Error("with 8 weeks of history: want ok=false")
	}
	if _, ok := weeklyConversionLine(before[len(before)-44:], today, 101, 99); !ok {
		t.Error("with exactly 9 weeks of history: want ok=true")
	}
}

// evaluate runs Ichimoku on candles with the market closed, so the last candle
// is treated as today, as in the backtester.
func evaluate(t *testing.T, candles []kiteconnect.HistoricalData) (bool, string) {
	t.Helper()
	ctx := domain.NewMarketContext()
	ctx.Set(1, candles)
	ok, sig := NewIchimokuKinkoHyo().Evaluate(models.Tick{InstrumentToken: 1}, ctx)
	return ok, sig.StrategyName
}

// ichimokuSeries is 130 flat sessions at 100 ending on Friday 2026-06-12, with
// the last session left for the test to shape.
func ichimokuSeries() []kiteconnect.HistoricalData {
	return flat(weekdays(istDay(2026, time.June, 12), 130), 100)
}

func TestIchimokuFutureBuy(t *testing.T) {
	// A flat stock breaks out of the cloud today: open 100, close 120. That is
	// also above May's high (100), for the first time in June, and above the
	// weekly CL ((120+100)/2 = 110).
	candles := ichimokuSeries()
	last := len(candles) - 1
	candles[last].High, candles[last].Close = 120, 120

	if ok, name := evaluate(t, candles); !ok || name != "IchimokuKinkoHyo (Future BUY)" {
		t.Fatalf("got %v %q, want IchimokuKinkoHyo (Future BUY)", ok, name)
	}

	// June already went above May's high two sessions ago, so the monthly break
	// isn't fresh: plain BUY instead.
	candles[last-2].High = 101
	if ok, name := evaluate(t, candles); !ok || name != "IchimokuKinkoHyo (BUY)" {
		t.Fatalf("with an earlier June break: got %v %q, want IchimokuKinkoHyo (BUY)", ok, name)
	}
}

func TestIchimokuFutureSell(t *testing.T) {
	// The mirror: open 100, close 80, below the cloud and May's low for the
	// first time in June. A spike to 105 fifteen sessions ago keeps the Base
	// Line above the Conversion Line and the weekly CL at (105+80)/2 = 92.5.
	candles := ichimokuSeries()
	last := len(candles) - 1
	candles[last].Low, candles[last].Close = 80, 80
	candles[last-15].High = 105

	if ok, name := evaluate(t, candles); !ok || name != "IchimokuKinkoHyo (Future SELL)" {
		t.Fatalf("got %v %q, want IchimokuKinkoHyo (Future SELL)", ok, name)
	}

	candles[last-2].Low = 99
	if ok, name := evaluate(t, candles); !ok || name != "IchimokuKinkoHyo (SELL)" {
		t.Fatalf("with an earlier June break: got %v %q, want IchimokuKinkoHyo (SELL)", ok, name)
	}
}
