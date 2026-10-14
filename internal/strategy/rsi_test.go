package strategy

import (
	"math"
	"strings"
	"testing"
	"time"

	"algotrader/internal/domain"
)

// Weekly and monthly RSI must bucket sessions by the IST calendar even when a
// candle's time is in UTC, as candles read back from SQLite are: an IST-midnight
// session is 18:30 UTC the day before. Otherwise every Monday joins the previous
// week and every 1st joins the previous month.
func TestResampleClosesUsesISTCalendar(t *testing.T) {
	utc := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, domain.IST).UTC() }
	series := []dailyClose{
		{Date: utc(2026, time.September, 18), Close: 10}, // Fri, ISO week 38
		{Date: utc(2026, time.September, 21), Close: 20}, // Mon, week 39
		{Date: utc(2026, time.September, 30), Close: 30}, // Wed, week 40, September
		{Date: utc(2026, time.October, 1), Close: 40},    // Thu, week 40, October
	}

	if got, want := resampleCloses(series, weekBucket), []float64{10, 20, 40}; !equalCloses(got, want) {
		t.Errorf("weekly closes = %v, want %v", got, want)
	}
	if got, want := resampleCloses(series, monthBucket), []float64{30, 40}; !equalCloses(got, want) {
		t.Errorf("monthly closes = %v, want %v", got, want)
	}
}

func equalCloses(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// day builds a dailyClose for a calendar date, keeping the tests readable.
func day(y int, m time.Month, d int, close float64) dailyClose {
	return dailyClose{Date: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Close: close}
}

func TestResampleClosesWeekly(t *testing.T) {
	// 2024-01-01 is a Monday, so each Monâ€“Fri block is one ISO week.
	series := []dailyClose{
		day(2024, time.January, 1, 10), // ISO week 1
		day(2024, time.January, 3, 11),
		day(2024, time.January, 5, 12), // week 1 closes here
		day(2024, time.January, 8, 13), // ISO week 2
		day(2024, time.January, 12, 15),
	}

	got := resampleCloses(series, weekBucket)
	want := []float64{12, 15}

	if len(got) != len(want) {
		t.Fatalf("resampleCloses weekly: got %d closes %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("resampleCloses weekly [%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}

// A week spanning a year boundary must stay one bucket â€” that is precisely what
// ISO week numbering buys us over (year, week-of-year).
func TestResampleClosesWeeklyAcrossYearBoundary(t *testing.T) {
	series := []dailyClose{
		day(2024, time.December, 30, 50), // Mon, ISO week 1 of 2025
		day(2025, time.January, 2, 55),   // Thu, same ISO week
		day(2025, time.January, 6, 60),   // Mon, next ISO week
	}

	got := resampleCloses(series, weekBucket)
	if len(got) != 2 {
		t.Fatalf("expected 2 weekly closes across the year boundary, got %d: %v", len(got), got)
	}
	if got[0] != 55 || got[1] != 60 {
		t.Errorf("got %v, want [55 60]", got)
	}
}

func TestResampleClosesMonthly(t *testing.T) {
	series := []dailyClose{
		day(2024, time.January, 2, 18),
		day(2024, time.January, 31, 20), // January closes here
		day(2024, time.February, 1, 21),
		day(2024, time.February, 29, 25), // February closes here
		day(2024, time.March, 1, 26),     // in-progress month
	}

	got := resampleCloses(series, monthBucket)
	want := []float64{20, 25, 26}

	if len(got) != len(want) {
		t.Fatalf("resampleCloses monthly: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("resampleCloses monthly [%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestResampleClosesEmpty(t *testing.T) {
	if got := resampleCloses(nil, weekBucket); got != nil {
		t.Errorf("expected nil for an empty series, got %v", got)
	}
}

func TestRSIFromCloses(t *testing.T) {
	rising := make([]float64, 0, 20)
	falling := make([]float64, 0, 20)
	for i := 0; i < 20; i++ {
		rising = append(rising, float64(100+i))
		falling = append(falling, float64(100-i))
	}

	if got := rsiFromCloses(rising, 14); got != 100 {
		t.Errorf("an unbroken advance should read 100, got %v", got)
	}
	if got := rsiFromCloses(falling, 14); got != 0 {
		t.Errorf("an unbroken decline should read 0, got %v", got)
	}

	// Too short to seed the average â€” reported as unknown, not as oversold.
	if got := rsiFromCloses(rising[:14], 14); got != 0 {
		t.Errorf("a series of exactly `period` closes cannot seed an RSI, got %v", got)
	}
}

// The Advanced GFS daily leg: band 57â€“63, turning up above 60.
func TestAdvancedGFSDailyLeg(t *testing.T) {
	cases := []struct {
		name      string
		prev, rsi float64
		want      bool
	}{
		{"turns up from mid-band", 59, 61, true},
		{"turns up from the band floor", 57, 60.5, true},
		{"turns up from the band ceiling", 63, 64, true},
		{"prev below the band", 56.9, 62, false},
		{"prev above the band", 63.1, 65, false},
		{"today lower than yesterday", 62, 61, false},
		{"today equal to yesterday", 61, 61, false},
		{"rising but still under the level", 57, 58, false},
		{"rising to exactly the level is not above it", 57, 60, false},
		{"unknown prevRSI during warm-up", 0, 62, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := turnedUpAt(tc.prev, tc.rsi, advancedGFSBand); got != tc.want {
				t.Errorf("turnedUpAt(%v, %v, advancedGFSBand) = %v, want %v", tc.prev, tc.rsi, got, tc.want)
			}
		})
	}
}

// The GFS daily leg: band 37â€“44, turning up above 40.
func TestGFSDailyLeg(t *testing.T) {
	cases := []struct {
		name      string
		prev, rsi float64
		want      bool
	}{
		{"turns up from mid-band", 39, 41, true},
		{"turns up from the band floor", 37, 40.5, true},
		{"turns up from the band ceiling", 44, 45, true},
		{"prev below the band", 36.9, 42, false},
		{"prev above the band", 44.1, 46, false},
		{"today lower than yesterday", 42, 41, false},
		{"today equal to yesterday", 41, 41, false},
		{"rising but still under the level", 37, 39, false},
		{"rising to exactly the level is not above it", 37, 40, false},
		{"unknown prevRSI during warm-up", 0, 42, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := turnedUpAt(tc.prev, tc.rsi, gfsBand); got != tc.want {
				t.Errorf("turnedUpAt(%v, %v, gfsBand) = %v, want %v", tc.prev, tc.rsi, got, tc.want)
			}
		})
	}
}

// The bands must not overlap, otherwise a single bar could satisfy both legs and
// the choice of signal name would come down to evaluation order.
func TestGFSBandsAreDisjoint(t *testing.T) {
	if gfsBand.Upper >= advancedGFSBand.Lower {
		t.Fatalf("bands overlap: GFS upper %v, Advanced GFS lower %v", gfsBand.Upper, advancedGFSBand.Lower)
	}
	for rsi := 0.0; rsi <= 100.0; rsi += 0.5 {
		for prev := 0.0; prev <= 100.0; prev += 0.5 {
			if turnedUpAt(prev, rsi, gfsBand) && turnedUpAt(prev, rsi, advancedGFSBand) {
				t.Fatalf("both legs fired for prev=%v rsi=%v", prev, rsi)
			}
		}
	}
}

// Each leg is memoryless, so it can fire on back-to-back sessions while RSI
// grinds up through its band. That is by design; de-duplication is the caller's
// job (StateRegistry live, -cooldown in the backtester).
func TestDailyLegRepeatsWithinTheBand(t *testing.T) {
	readings := []float64{38, 40.5, 42, 43.5}
	fired := 0
	for i := 1; i < len(readings); i++ {
		if turnedUpAt(readings[i-1], readings[i], gfsBand) {
			fired++
		}
	}
	if fired != 3 {
		t.Errorf("expected the memoryless trigger to fire on all 3 rising bars, got %d", fired)
	}
}

// The series must agree with the single-value calculation at every point, since
// the GFS legs used to read rsiFromCloses and now read the series.
func TestRSISeriesMatchesRSIFromCloses(t *testing.T) {
	prices := []float64{100, 102, 101, 105, 104, 103, 108, 110, 107, 106, 111, 115, 113, 112, 118, 117, 120, 116, 119, 121}
	series := rsiSeries(prices, 14)
	if len(series) != len(prices)-14 {
		t.Fatalf("expected %d values, got %d", len(prices)-14, len(series))
	}
	for i, got := range series {
		want := rsiFromCloses(prices[:14+i+1], 14)
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("series[%d] = %v, rsiFromCloses = %v", i, got, want)
		}
	}
	if rsiSeries(prices[:14], 14) != nil {
		t.Error("a series of exactly `period` closes cannot seed an RSI")
	}
}

func TestRangeStateOf(t *testing.T) {
	cases := []struct {
		name string
		hist []float64
		want rsiRange
	}{
		{"empty history", nil, rangeSideways},
		{"never broke out of 38â€“61", []float64{45, 52, 61, 38, 41}, rangeSideways},
		{"above 61 is bull", []float64{45, 62, 52, 48}, rangeBull},
		{"below 38 is bear", []float64{65, 37, 47, 55, 61}, rangeBear},
		{"bear holding 40 after leaving it shifts to sideways", []float64{30, 48, 39, 42}, rangeSideways},
		{"bull failing at 60 after leaving it shifts to sideways", []float64{70, 55, 61, 58}, rangeSideways},
		{"sideways breaks up into bull", []float64{30, 48, 39, 42, 60, 41, 66}, rangeBull},
		{"sideways breaks down into bear", []float64{70, 55, 61, 58, 40, 60, 35}, rangeBear},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rangeStateOf(tc.hist).regime; got != tc.want {
				t.Errorf("rangeStateOf(%v) = %v, want %v", tc.hist, got, tc.want)
			}
		})
	}
}

func TestRangeEventAt(t *testing.T) {
	cases := []struct {
		name  string
		hist  []float64 // up to and including yesterday
		today float64
		want  rangeEvent
	}{
		// Bullish range-shift: bear, RSI has left the 40 zone, comes back and holds it.
		{"bear range holds 40 after leaving it", []float64{30, 48, 39}, 42, bullishShift},
		{"bear range first rise through 40 is not a shift", []float64{30, 35, 39}, 42, noEvent},
		{"bear range back at 40 but still falling", []float64{30, 48, 42}, 39, strongBearish},
		{"crossing 60 out of a bear range is no longer a signal", []float64{65, 52, 35, 47, 55, 58}, 62, noEvent},

		// Strong Bullish: bull range, support at 57â€“63 holds and turns up.
		{"support holds at 60 and turns up", []float64{74, 67, 59}, 63, strongBullish},
		{"support zone but still falling", []float64{74, 67, 59}, 55, noEvent},
		{"turns up but stays under 60", []float64{74, 67, 57}, 59, noEvent},
		{"pullback too shallow to reach support", []float64{74, 70, 66}, 68, noEvent},

		// Bearish range-shift: bull, RSI has left the 60 zone, comes back and fails at it.
		{"bull range fails at 60 after leaving it", []float64{70, 55, 61}, 58, bearishShift},
		{"bull range first drop through 60 is not a shift", []float64{70, 66, 61}, 58, noEvent},
		{"crossing 40 out of a bull range is no longer a signal", []float64{35, 66, 56, 47, 42}, 37, noEvent},

		// Strong Bearish: bear range, resistance at 37â€“43 holds and turns down.
		{"resistance holds at 40 and turns down", []float64{26, 33, 41}, 37, strongBearish},
		{"resistance zone but still rising", []float64{26, 33, 41}, 48, noEvent},
		{"turns down but stays over 40", []float64{26, 33, 43}, 41, noEvent},

		// Sideways never signals.
		{"sideways turning up at 40", []float64{30, 48, 39, 42, 55, 39}, 42, noEvent},
		{"sideways turning down at 60", []float64{30, 48, 39, 42, 55, 60}, 57, noEvent},
		{"no range established yet", []float64{45, 50, 55}, 62, noEvent},
		{"no history", nil, 62, noEvent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rangeEventAt(tc.hist, tc.today); got != tc.want {
				t.Errorf("rangeEventAt(%v, %v) = %v, want %v", tc.hist, tc.today, got, tc.want)
			}
		})
	}
}

// TestRangeCycle walks the RSI range-shift diagram end to end: bear â†’ (1) Bullish
// Range-Shift â†’ sideways â†’ bull â†’ (2) Strong Bullish â†’ (3) Bearish Range-Shift â†’
// sideways â†’ bear â†’ (4) Strong Bearish. Nothing else may fire.
func TestRangeCycle(t *testing.T) {
	path := []float64{
		30, 20, 35, 25, // bear
		48, 43, 41, 44, // leaves 40, pulls back, holds: (1)
		55, 60, 50, 40, 45, // sideways: resistance at 60, support at 40
		70, 65, 59, 64, // breaks into bull, holds 60: (2)
		80, 70, 85, 75, 55, // bull, then drops out of the 60 zone
		60, 56, // fails at 60: (3)
		48, 40, 50, 60, 50, // sideways
		30, 25, 40, 36, // breaks into bear, fails at 40: (4)
		28, 33, 25,
	}
	want := map[int]rangeEvent{7: bullishShift, 16: strongBullish, 23: bearishShift, 32: strongBearish}
	for i := 1; i < len(path); i++ {
		if got := rangeEventAt(path[:i], path[i]); got != want[i] {
			t.Errorf("bar %d (RSI %v â†’ %v): got %v, want %v", i, path[i-1], path[i], got, want[i])
		}
	}
}

// The bearish GFS legs mirror the bullish ones around 50.
func TestBearishGFSDailyLegs(t *testing.T) {
	cases := []struct {
		name      string
		band      rsiBand
		prev, rsi float64
		want      bool
	}{
		{"Bearish GFS turns down through 60", bearishGFSBand, 61, 59, true},
		{"Bearish GFS from the band floor", bearishGFSBand, 56, 55, true},
		{"Bearish GFS prev below the band", bearishGFSBand, 55.9, 50, false},
		{"Bearish GFS falling but still over 60", bearishGFSBand, 63, 61, false},
		{"Bearish Advanced GFS turns down through 40", bearishAdvancedGFSBand, 41, 38, true},
		{"Bearish Advanced GFS prev above the band", bearishAdvancedGFSBand, 43.1, 38, false},
		{"Bearish Advanced GFS rising", bearishAdvancedGFSBand, 39, 41, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := turnedDownAt(tc.prev, tc.rsi, tc.band); got != tc.want {
				t.Errorf("turnedDownAt(%v, %v) = %v, want %v", tc.prev, tc.rsi, got, tc.want)
			}
		})
	}
}

func TestSignalNames(t *testing.T) {
	r := NewRSI(14)
	bullMTF := rsiReadings{Weekly: 66, WeeklyOK: true, Monthly: 69, MonthlyOK: true}
	bearMTF := rsiReadings{Weekly: 33, WeeklyOK: true, Monthly: 30, MonthlyOK: true}

	with := func(base rsiReadings, f func(*rsiReadings)) rsiReadings { f(&base); return base }

	cases := []struct {
		name string
		x    rsiReadings
		want []string
	}{
		{
			"weekly and daily range-shifts on the same day, weekly first",
			with(rsiReadings{Monthly: 50, MonthlyOK: true}, func(x *rsiReadings) {
				x.WeeklyHist, x.Weekly, x.WeeklyOK = []float64{30, 48, 39}, 42, true
				x.DailyHist, x.Daily = []float64{30, 47, 41}, 44
			}),
			[]string{"RSI (Weekly Bullish Range-Shift)", "RSI (Daily Bullish Range-Shift)"},
		},
		{
			"Advanced GFS replaces daily Strong Bullish",
			with(bullMTF, func(x *rsiReadings) { x.DailyHist, x.Daily = []float64{74, 67, 59}, 63 }),
			[]string{"RSI (Advanced GFS)"},
		},
		{
			"daily Strong Bullish without higher-timeframe confirmation",
			with(rsiReadings{Weekly: 55, WeeklyOK: true, Monthly: 69, MonthlyOK: true}, func(x *rsiReadings) {
				x.DailyHist, x.Daily = []float64{74, 67, 59}, 63
			}),
			[]string{"RSI (Daily Strong Bullish)"},
		},
		{
			"GFS turning up at 40",
			with(bullMTF, func(x *rsiReadings) { x.DailyHist, x.Daily = []float64{65, 50, 39}, 41 }),
			[]string{"RSI (GFS)"},
		},
		{
			"Bearish Advanced GFS replaces daily Strong Bearish",
			with(bearMTF, func(x *rsiReadings) { x.DailyHist, x.Daily = []float64{26, 33, 41}, 37 }),
			[]string{"RSI (Bearish Advanced GFS)"},
		},
		{
			"Bearish GFS turning down at 60",
			with(bearMTF, func(x *rsiReadings) { x.DailyHist, x.Daily = []float64{30, 50, 61}, 59 }),
			[]string{"RSI (Bearish GFS)"},
		},
		{
			"an unknown monthly RSI never confirms a bearish GFS",
			with(rsiReadings{Weekly: 33, WeeklyOK: true}, func(x *rsiReadings) {
				x.DailyHist, x.Daily = []float64{30, 50, 61}, 59
			}),
			nil,
		},
		{
			"weekly Strong Bearish",
			with(rsiReadings{Monthly: 50, MonthlyOK: true}, func(x *rsiReadings) {
				x.WeeklyHist, x.Weekly, x.WeeklyOK = []float64{25, 34, 42}, 38, true
				x.DailyHist, x.Daily = []float64{45, 47, 48}, 46
			}),
			[]string{"RSI (Weekly Strong Bearish)"},
		},
		{"nothing fires", with(bullMTF, func(x *rsiReadings) { x.DailyHist, x.Daily = []float64{70, 72}, 74 }), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.signalNames(tc.x)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("signalNames = %q, want %q", got, tc.want)
			}
		})
	}
}

// Every name the strategy can emit must be bullish or bearish by the backtester's
// vocabulary, so the new bearish signals are scored as shorts.
func TestSignalNamesCarryDirection(t *testing.T) {
	for _, name := range RSISignalNames() {
		bearish := strings.Contains(strings.ToUpper(name), "BEARISH")
		wantBearish := strings.Contains(name, "Bearish")
		if bearish != wantBearish {
			t.Errorf("%q: direction marker mismatch", name)
		}
	}
	if n := len(RSISignalNames()); n != 12 {
		t.Errorf("expected 12 RSI signal names, got %d", n)
	}
}

func TestReadingsOfCarriesKnownTimeframesOnly(t *testing.T) {
	got := readingsOf(rsiReadings{Daily: 61.5, Weekly: 64, WeeklyOK: true, Monthly: 0, MonthlyOK: false})
	if got.Daily != 61.5 || got.Weekly != 64 || !got.WeeklyOK || got.MonthlyOK {
		t.Errorf("readingsOf = %+v", got)
	}
}
