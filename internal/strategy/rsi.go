package strategy

import (
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	"github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
)

// RSI emits twelve signals in two families.
//
// GFS (Grandfather–Father–Son): the weekly and monthly RSI agree on the trend,
// and the daily RSI turns in that direction at a pivot level.
//
//	RSI (GFS)                   weekly & monthly > 60, daily turns up at 40
//	RSI (Advanced GFS)          weekly & monthly > 60, daily turns up at 60
//	RSI (Bearish GFS)           weekly & monthly < 40, daily turns down at 60
//	RSI (Bearish Advanced GFS)  weekly & monthly < 40, daily turns down at 40
//
// Range signals: RSI lives in a bull range (40–80) in an uptrend and a bear
// range (20–60) in a downtrend. Each of these is emitted for the daily and the
// weekly RSI ("RSI (Daily ...)" / "RSI (Weekly ...)"):
//
//	Bullish Range-Shift  bear range, RSI crosses above 60 — downtrend turning up
//	Strong Bullish       bull range, RSI holds support at 57–63 and turns up
//	Bearish Range-Shift  bull range, RSI crosses below 40 — uptrend turning down
//	Strong Bearish       bear range, RSI meets resistance at 37–43 and turns down
//
// Advanced GFS is a daily Strong Bullish confirmed by both higher timeframes,
// so when both fire only Advanced GFS is emitted; likewise Bearish Advanced GFS
// replaces a daily Strong Bearish.
//
// The strategy is stateless — the range a stock is in is re-derived from its RSI
// history on every evaluation. One instance is safe to share across all
// instruments and all worker goroutines.
type RSI struct {
	Period int

	// WeeklyPeriod and MonthlyPeriod are the RSI lookbacks applied to the
	// resampled higher-timeframe series. They default to Period.
	WeeklyPeriod  int
	MonthlyPeriod int

	// MTFLevel is the RSI floor both higher timeframes must clear for a bullish
	// GFS. The bearish GFS mirrors it: both must sit below 100 − MTFLevel.
	MTFLevel float64
}

func NewRSI(period int) *RSI {
	if period <= 0 {
		period = 14 // default Wilder's smoothing period
	}
	return &RSI{
		Period:        period,
		WeeklyPeriod:  period,
		MonthlyPeriod: period,
		MTFLevel:      60,
	}
}

func (r *RSI) Name() string { return "RSI" }

// rsiBand describes a turn at a pivot: the band yesterday's RSI must sit in, and
// the level today's RSI must be beyond after turning.
type rsiBand struct {
	Lower float64
	Upper float64
	Level float64
}

var (
	// gfsBand is the daily leg of GFS: 40 as the pivot.
	gfsBand = rsiBand{Lower: 37, Upper: 44, Level: 40}

	// advancedGFSBand is the daily leg of Advanced GFS: 60 as the pivot.
	advancedGFSBand = rsiBand{Lower: 57, Upper: 63, Level: 60}

	// bearishGFSBand and bearishAdvancedGFSBand mirror the bullish legs around 50.
	bearishGFSBand         = rsiBand{Lower: 56, Upper: 63, Level: 60}
	bearishAdvancedGFSBand = rsiBand{Lower: 37, Upper: 43, Level: 40}

	// supportZone is where a pullback in the bull range must hold for Strong
	// Bullish; resistanceZone is where a rally in the bear range must fail for
	// Strong Bearish.
	supportZone    = rsiBand{Lower: 57, Upper: 63, Level: 60}
	resistanceZone = rsiBand{Lower: 37, Upper: 43, Level: 40}
)

const (
	// bullRangeMark and bearRangeMark are the extremes that place a stock in a
	// range: touching 60 or higher puts it in the bull range, 40 or lower in the
	// bear range. Readings between them leave the range unchanged.
	bullRangeMark = 60.0
	bearRangeMark = 40.0
)

// turnedUpAt reports whether RSI turned up at this band's level on the bar being
// evaluated:
//
//	yesterday sat inside the band straddling the level — at it, neither
//	extended above nor broken below;
//	today closed higher than yesterday — the turn is up, not down;
//	today is above the level — the turn happened on the bullish side of it.
//
// The test is memoryless: it reads two consecutive bars and nothing else, so it
// can repeat on consecutive sessions while RSI grinds up through the band.
// De-duplication belongs to the caller (StateRegistry live, -cooldown in the
// backtester).
func turnedUpAt(prevRSI, rsi float64, b rsiBand) bool {
	return prevRSI >= b.Lower && prevRSI <= b.Upper && rsi > prevRSI && rsi > b.Level
}

// turnedDownAt is the bearish mirror of turnedUpAt: yesterday inside the band,
// today lower than yesterday and below the level.
func turnedDownAt(prevRSI, rsi float64, b rsiBand) bool {
	return prevRSI >= b.Lower && prevRSI <= b.Upper && rsi < prevRSI && rsi < b.Level
}

// rsiRange is the range a stock's RSI is trading in.
type rsiRange int8

const (
	rangeUnknown rsiRange = iota // RSI has never left the 40–60 overlap
	rangeBull                    // last extreme was 60 or higher
	rangeBear                    // last extreme was 40 or lower
)

// priorRange walks an RSI history back to its most recent extreme and reports
// the range that extreme placed the stock in.
func priorRange(hist []float64) rsiRange {
	for i := len(hist) - 1; i >= 0; i-- {
		switch {
		case hist[i] >= bullRangeMark:
			return rangeBull
		case hist[i] <= bearRangeMark:
			return rangeBear
		}
	}
	return rangeUnknown
}

// rangeEvent is a range signal on one timeframe.
type rangeEvent int8

const (
	noEvent rangeEvent = iota
	bullishShift
	strongBullish
	bearishShift
	strongBearish
)

// rangeEventAt classifies today's RSI against the history up to and including
// yesterday. The four events need different ranges or opposite moves, so at
// most one can fire.
func rangeEventAt(hist []float64, rsi float64) rangeEvent {
	if len(hist) == 0 {
		return noEvent
	}
	prev := hist[len(hist)-1]

	switch priorRange(hist) {
	case rangeBear:
		if prev <= bullRangeMark && rsi > bullRangeMark {
			return bullishShift
		}
		if turnedDownAt(prev, rsi, resistanceZone) {
			return strongBearish
		}
	case rangeBull:
		if prev >= bearRangeMark && rsi < bearRangeMark {
			return bearishShift
		}
		if turnedUpAt(prev, rsi, supportZone) {
			return strongBullish
		}
	}
	return noEvent
}

// rangeEventNames maps a range event to its signal-name suffix.
var rangeEventNames = map[rangeEvent]string{
	bullishShift:  "Bullish Range-Shift",
	strongBullish: "Strong Bullish",
	bearishShift:  "Bearish Range-Shift",
	strongBearish: "Strong Bearish",
}

// RSISignalNames lists every signal name the RSI strategy can emit, in priority
// order. The engine uses it as the Discord allowlist.
func RSISignalNames() []string {
	var out []string
	for _, tf := range []string{"Weekly", "Daily"} {
		for _, ev := range []rangeEvent{strongBullish, bullishShift, strongBearish, bearishShift} {
			out = append(out, "RSI ("+tf+" "+rangeEventNames[ev]+")")
		}
	}
	return append(out,
		"RSI (Advanced GFS)", "RSI (GFS)",
		"RSI (Bearish Advanced GFS)", "RSI (Bearish GFS)",
	)
}

// rsiReadings is everything the signal rules read. The Hist slices run up to and
// including yesterday; the scalar beside each is today's reading.
type rsiReadings struct {
	DailyHist []float64
	Daily     float64

	// WeeklyHist is the weekly RSI as it stood yesterday: completed weeks plus
	// the running week closed at yesterday's price.
	WeeklyHist []float64
	Weekly     float64
	WeeklyOK   bool

	Monthly   float64
	MonthlyOK bool
}

// signalNames applies every rule to one set of readings and returns the signals
// that fire, highest priority first: weekly before daily, then GFS.
func (r *RSI) signalNames(x rsiReadings) []string {
	var names []string

	if x.WeeklyOK {
		if ev := rangeEventAt(x.WeeklyHist, x.Weekly); ev != noEvent {
			names = append(names, "RSI (Weekly "+rangeEventNames[ev]+")")
		}
	}

	daily := rangeEventAt(x.DailyHist, x.Daily)

	var prev float64
	if len(x.DailyHist) > 0 {
		prev = x.DailyHist[len(x.DailyHist)-1]
	}
	// An unknown higher timeframe must never be read as confirmation, in either
	// direction.
	known := x.WeeklyOK && x.MonthlyOK && len(x.DailyHist) > 0
	mtfBull := known && x.Weekly > r.MTFLevel && x.Monthly > r.MTFLevel
	mtfBear := known && x.Weekly < 100-r.MTFLevel && x.Monthly < 100-r.MTFLevel

	advanced := mtfBull && turnedUpAt(prev, x.Daily, advancedGFSBand)
	classic := mtfBull && turnedUpAt(prev, x.Daily, gfsBand)
	bearAdvanced := mtfBear && turnedDownAt(prev, x.Daily, bearishAdvancedGFSBand)
	bearClassic := mtfBear && turnedDownAt(prev, x.Daily, bearishGFSBand)

	// Advanced GFS is the confirmed form of a daily Strong Bullish — send one alert.
	if (advanced && daily == strongBullish) || (bearAdvanced && daily == strongBearish) {
		daily = noEvent
	}
	if daily != noEvent {
		names = append(names, "RSI (Daily "+rangeEventNames[daily]+")")
	}

	if advanced {
		names = append(names, "RSI (Advanced GFS)")
	}
	if classic {
		names = append(names, "RSI (GFS)")
	}
	if bearAdvanced {
		names = append(names, "RSI (Bearish Advanced GFS)")
	}
	if bearClassic {
		names = append(names, "RSI (Bearish GFS)")
	}
	return names
}

// rsiSeries computes Wilder's RSI at every point of a closing-price series. The
// result is tail-aligned: out[i] is the RSI at prices[period+i], and
// out[len(out)-1] equals rsiFromCloses(prices, period). It returns nil when the
// series is too short to seed the average.
func rsiSeries(prices []float64, period int) []float64 {
	if period <= 0 || len(prices) <= period {
		return nil
	}

	var sumGain, sumLoss float64
	for i := 1; i <= period; i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			sumGain += change
		} else {
			sumLoss -= change
		}
	}

	avgGain := sumGain / float64(period)
	avgLoss := sumLoss / float64(period)

	out := make([]float64, 0, len(prices)-period)
	out = append(out, rsiFrom(avgGain, avgLoss))

	for i := period + 1; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		gain, loss := 0.0, 0.0
		if change > 0 {
			gain = change
		} else {
			loss = -change
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
		out = append(out, rsiFrom(avgGain, avgLoss))
	}
	return out
}

// rsiFromCloses computes Wilder's RSI over a closing-price series that already
// ends with the current price. It returns 0 when the series is too short to
// seed the average, which callers treat as "unknown" rather than "oversold".
func rsiFromCloses(prices []float64, period int) float64 {
	series := rsiSeries(prices, period)
	if len(series) == 0 {
		return 0
	}
	return series[len(series)-1]
}

// rsiFrom converts a smoothed gain/loss pair into an RSI reading.
func rsiFrom(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		return 100.0
	}
	return 100.0 - (100.0 / (1.0 + avgGain/avgLoss))
}

// dailyClose is one session's closing price with the date it belongs to.
// It is the input to weekly and monthly resampling.
type dailyClose struct {
	Date  time.Time
	Close float64
}

// resampleCloses collapses a daily close series into higher-timeframe closes by
// keeping the LAST close of each bucket, which is what a weekly or monthly
// candle closes at. The series must be in ascending date order.
//
// The final bucket is normally still in progress (mid-week or mid-month); its
// running close is included deliberately, matching how charting platforms show
// a live higher-timeframe bar.
func resampleCloses(series []dailyClose, bucketOf func(time.Time) (int, int)) []float64 {
	if len(series) == 0 {
		return nil
	}

	closes := make([]float64, 0, len(series)/5+1)
	curA, curB := bucketOf(series[0].Date)
	last := series[0].Close

	for _, bar := range series[1:] {
		a, b := bucketOf(bar.Date)
		if a != curA || b != curB {
			closes = append(closes, last) // previous bucket just completed
			curA, curB = a, b
		}
		last = bar.Close
	}
	return append(closes, last) // the in-progress bucket
}

// weekBucket keys a date by ISO year and ISO week, so weeks spanning a year
// boundary stay a single bucket.
func weekBucket(t time.Time) (int, int) { return t.ISOWeek() }

// monthBucket keys a date by calendar year and month.
func monthBucket(t time.Time) (int, int) { return t.Year(), int(t.Month()) }

// Evaluate returns the highest-priority signal, for callers that take only one.
func (r *RSI) Evaluate(tick models.Tick, ctx *domain.MarketContext) (bool, domain.OrderSignal) {
	all := r.EvaluateAll(tick, ctx)
	if len(all) == 0 {
		return false, domain.OrderSignal{}
	}
	return true, all[0]
}

// EvaluateAll returns every signal that fires on this tick, highest priority first.
func (r *RSI) EvaluateAll(tick models.Tick, ctx *domain.MarketContext) []domain.OrderSignal {
	rawCandles, ok := ctx.Get(tick.InstrumentToken)
	if !ok || len(rawCandles) < r.Period {
		return nil
	}

	// Copy the slice so we don't modify the shared array backing
	candles := make([]kiteconnect.HistoricalData, len(rawCandles))
	copy(candles, rawCandles)

	// Strip today's incomplete candle if the REST API returned it (intraday boot).
	loc, _ := time.LoadLocation("Asia/Kolkata")
	if loc == nil {
		loc = time.FixedZone("IST", 5*3600+30*60)
	}
	now := time.Now().In(loc)
	lastDate := candles[len(candles)-1].Date.In(loc)
	if lastDate.Year() == now.Year() && lastDate.Month() == now.Month() && lastDate.Day() == now.Day() {
		candles = candles[:len(candles)-1]
	}

	_, marketOpen := ctx.Intraday.Get(tick.InstrumentToken)
	var currentPrice float64
	var currentDate time.Time

	if marketOpen {
		currentPrice = tick.LastPrice
		currentDate = now
	} else {
		// Market is closed: use last candle close
		if len(candles) == 0 {
			return nil
		}
		last := candles[len(candles)-1]
		currentPrice = last.Close
		currentDate = last.Date.Time
		// We remove the last candle since we're using its close as the "current price"
		candles = candles[:len(candles)-1]
	}

	if len(candles) < r.Period {
		return nil
	}

	// series is the daily close series INCLUDING the bar being evaluated, with
	// currentPrice as its close. Every timeframe is computed from it.
	series := make([]dailyClose, 0, len(candles)+1)
	closes := make([]float64, 0, len(candles)+1)
	for _, c := range candles {
		series = append(series, dailyClose{Date: c.Date.Time, Close: c.Close})
		closes = append(closes, c.Close)
	}
	series = append(series, dailyClose{Date: currentDate, Close: currentPrice})
	closes = append(closes, currentPrice)

	daily := rsiSeries(closes, r.Period)
	if len(daily) < 2 {
		return nil
	}

	// The weekly RSI as it stood yesterday comes from the series without today's
	// bar, so a weekly signal fires on the day the weekly RSI makes its move.
	weeklyToday := rsiSeries(resampleCloses(series, weekBucket), r.WeeklyPeriod)
	weeklyPrev := rsiSeries(resampleCloses(series[:len(series)-1], weekBucket), r.WeeklyPeriod)
	monthly := rsiSeries(resampleCloses(series, monthBucket), r.MonthlyPeriod)

	x := rsiReadings{
		DailyHist:  daily[:len(daily)-1],
		Daily:      daily[len(daily)-1],
		WeeklyHist: weeklyPrev,
	}
	if len(weeklyToday) > 0 {
		x.Weekly, x.WeeklyOK = weeklyToday[len(weeklyToday)-1], true
	}
	if len(monthly) > 0 {
		x.Monthly, x.MonthlyOK = monthly[len(monthly)-1], true
	}

	names := r.signalNames(x)
	if len(names) == 0 {
		return nil
	}
	signalTime := time.Now().UTC()
	readings := readingsOf(x)
	signals := make([]domain.OrderSignal, 0, len(names))
	for _, name := range names {
		signals = append(signals, domain.OrderSignal{
			InstrumentToken: tick.InstrumentToken,
			StrategyName:    name,
			SignalTime:      signalTime,
			Strategy:        r.Name(),
			Price:           currentPrice,
			RSI:             readings,
		})
	}
	return signals
}

// readingsOf copies the readings a signal was based on, for the website and
// exports. Weekly and monthly are flagged unknown when there wasn't enough history.
func readingsOf(x rsiReadings) *domain.RSIReadings {
	return &domain.RSIReadings{
		Daily:     x.Daily,
		Weekly:    x.Weekly,
		WeeklyOK:  x.WeeklyOK,
		Monthly:   x.Monthly,
		MonthlyOK: x.MonthlyOK,
	}
}
