package strategy

import (
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	"github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
)

type IchimokuKinkoHyo struct {
	// rsi supplies the RSI readings attached to every Ichimoku signal. It uses
	// the same period as the RSI strategy the engine registers.
	rsi *RSI
}

func NewIchimokuKinkoHyo() *IchimokuKinkoHyo { return &IchimokuKinkoHyo{rsi: NewRSI(14)} }

func (i *IchimokuKinkoHyo) Name() string { return "IchimokuKinkoHyo" }

// Evaluate returns the first Ichimoku signal that fires on this tick, carrying
// the stock's daily, weekly and monthly RSI at that moment. RSI is only computed
// when a signal fires, so quiet ticks cost nothing extra.
func (i *IchimokuKinkoHyo) Evaluate(tick models.Tick, ctx *domain.MarketContext) (bool, domain.OrderSignal) {
	ok, sig := i.evaluate(tick, ctx)
	if ok && i.rsi != nil {
		sig.RSI = i.rsi.Readings(tick, ctx)
	}
	return ok, sig
}

// periodMidpoint returns (highest high + lowest low) / 2 over a candle slice,
// seeded with today's live intraday high/low so the current session is included.
func periodMidpoint(candles []kiteconnect.HistoricalData, seedHigh, seedLow float64) float64 {
	high, low := seedHigh, seedLow
	for _, c := range candles {
		if c.High > high {
			high = c.High
		}
		if c.Low < low {
			low = c.Low
		}
	}
	return (high + low) / 2.0
}

func (i *IchimokuKinkoHyo) evaluate(tick models.Tick, ctx *domain.MarketContext) (bool, domain.OrderSignal) {

	// ── 1. Data retrieval ─────────────────────────────────────────────────────

	rawCandles, ok := ctx.Get(tick.InstrumentToken)
	if !ok || len(rawCandles) < 105 {
		return false, domain.OrderSignal{}
	}

	// Copy the slice so we don't modify the shared array backing
	candles := make([]kiteconnect.HistoricalData, len(rawCandles))
	copy(candles, rawCandles)

	// Strip today's incomplete candle if the REST API returned it (intraday boot).
	// This ensures `candles` always ends with yesterday's completed daily candle.
	var trueTodayOpen float64
	loc, _ := time.LoadLocation("Asia/Kolkata")
	if loc == nil {
		loc = time.FixedZone("IST", 5*3600+30*60)
	}
	now := time.Now().In(loc)
	lastDate := candles[len(candles)-1].Date.In(loc)
	if lastDate.Year() == now.Year() && lastDate.Month() == now.Month() && lastDate.Day() == now.Day() {
		trueTodayOpen = candles[len(candles)-1].Open
		candles = candles[:len(candles)-1]
	}

	// 105 minimum: 104 for Leading Span B + 1 extra for cross detection
	if len(candles) < 105 {
		return false, domain.OrderSignal{}
	}

	// ── Live vs. closed-market detection ─────────────────────────────────────
	// ctx.Intraday.Get returns false when no tick has been received today,
	// which means the market is closed (holiday / weekend / pre-open).
	// In that case we use the last closed candle as the "today" seed so that
	// all period counts remain correct without corrupting the midpoint.
	intraday, marketOpen := ctx.Intraday.Get(tick.InstrumentToken)

	var todayHigh, todayLow, todayOpen float64
	var conversionCandles, baseCandles []kiteconnect.HistoricalData
	var yesterday kiteconnect.HistoricalData
	var cnt int

	var currentPrice float64
	var yesterdayClose float64

	// history is every completed session before "today", and today is its date;
	// the monthly and weekly checks of the Future signals need both.
	var history []kiteconnect.HistoricalData
	var today time.Time

	if marketOpen {
		// Market is running: use live tick price and live intraday H/L seed.
		// candles[len-1] = yesterday's completed daily candle → correct "yesterday".
		currentPrice = tick.LastPrice
		todayHigh = intraday.High
		todayLow = intraday.Low
		yesterday = candles[len(candles)-1]
		if trueTodayOpen > 0 {
			todayOpen = trueTodayOpen
		} else {
			todayOpen = intraday.Open
		}
		yesterdayClose = candles[len(candles)-1].Close // last closed day = yesterday ✓
		conversionCandles = candles[len(candles)-8:]
		baseCandles = candles[len(candles)-25:]
		cnt = 25
		history = candles
		today = now
	} else {
		// Market is closed: treat candles[len-1] as "today" (the most recent
		// session) so all Ichimoku period counts stay correct.
		// Therefore "yesterday" = candles[len-2].
		last := candles[len(candles)-1]
		currentPrice = last.Close
		todayHigh = last.High
		todayLow = last.Low
		todayOpen = last.Open
		yesterday = candles[len(candles)-2]
		yesterdayClose = candles[len(candles)-2].Close // one session earlier ✓
		conversionCandles = candles[len(candles)-9:]
		baseCandles = candles[len(candles)-26:]
		cnt = 26
		history = candles[:len(candles)-1]
		today = last.Date.Time
	}

	// ── 2. Conversion Line (Tenkan-sen) — 9-period midpoint ──────────────────
	// "The average of the 9-period high and the 9-period low"
	conversionLine := periodMidpoint(conversionCandles, todayHigh, todayLow)

	// Previous Conversion Line (one session back) — needed for crossover detection
	//yesterday := candles[len(candles)-1]
	prevConversionLine := periodMidpoint(candles[len(candles)-9:len(candles)-1], yesterday.High, yesterday.Low)

	// ── 3. Base Line (Kijun-sen) — 26-period midpoint ────────────────────────
	// "The average of the 26-period high and the 26-period low"
	baseLine := periodMidpoint(baseCandles, todayHigh, todayLow)

	prevBaseLine := periodMidpoint(candles[len(candles)-26:len(candles)-1], yesterday.High, yesterday.Low)

	// ── 4. Leading Span A (Senkou Span A) ────────────────────────────────────
	// "The average of the Conversion Line and the Base Line, plotted 26 periods ahead"
	// To get the cloud visible TODAY, we use the Conversion Line and Base Line
	// that were calculated 26 sessions ago, seeded by that session's candle.
	sessionMinus26 := candles[len(candles)-cnt] // the "today" of 26 sessions ago
	sessionMinus27 := candles[len(candles)-(cnt+1)]
	// pastConversionLine: 8 closed candles before sessionMinus26 + sessionMinus26 seed = 9 periods
	pastConversionLine := periodMidpoint(candles[len(candles)-(cnt+8):len(candles)-cnt], sessionMinus26.High, sessionMinus26.Low)
	pastPrevConversionLine := periodMidpoint(candles[len(candles)-(cnt+9):len(candles)-(cnt+1)], sessionMinus27.High, sessionMinus27.Low)
	// pastBaseLine: 25 closed candles before sessionMinus26 + sessionMinus26 seed = 26 periods
	pastBaseLine := periodMidpoint(candles[len(candles)-(cnt+25):len(candles)-cnt], sessionMinus26.High, sessionMinus26.Low)
	leadingSpanA := (pastConversionLine + pastBaseLine) / 2.0

	sessionMinus52 := candles[len(candles)-(cnt+26)] // the "today" of 52 sessions ago
	pastConversionLine52 := periodMidpoint(candles[len(candles)-(cnt+26+8):len(candles)-(cnt+26)], sessionMinus52.High, sessionMinus52.Low)
	pastBaseLine52 := periodMidpoint(candles[len(candles)-(cnt+26+25):len(candles)-(cnt+26)], sessionMinus52.High, sessionMinus52.Low)
	leadingSpanA52 := (pastConversionLine52 + pastBaseLine52) / 2.0

	// ── 5. Leading Span B (Senkou Span B) ────────────────────────────────────
	// "The average of the 52-period high and the 52-period low, plotted 26 periods ahead"
	// Same shift logic as Leading Span A — use the window ending 26 sessions ago.
	// NOTE: strict calculation needs 78 candles (52 + 26 shift). With only 53
	// leadingSpanB: 51 closed candles before sessionMinus26 + sessionMinus26 seed = 52 periods
	leadingSpanB := periodMidpoint(candles[len(candles)-(cnt+51):len(candles)-cnt], sessionMinus26.High, sessionMinus26.Low)
	leadingSpanB52 := periodMidpoint(candles[len(candles)-(cnt+26+51):len(candles)-(cnt+26)], sessionMinus52.High, sessionMinus52.Low)
	// ── 6. Lagging Span (Chikou Span) ─────────────────────────────────────────
	// "The current closing price plotted 26 periods in the past"
	// Bullish when current price is above the high from 26 sessions ago
	// (i.e. the Lagging Span sits above historical price action)
	priceFrom26SessionsAgo := candles[len(candles)-(cnt+1)].High
	priceFrom27SessionsAgo := candles[len(candles)-(cnt+2)].High
	priceFrom26SessionsAgolow := candles[len(candles)-(cnt+1)].Low
	priceFrom27SessionsAgolow := candles[len(candles)-(cnt+2)].Low

	// ── 7. Cloud (Kumo) boundaries ───────────────────────────────────────────
	cloudTop := leadingSpanA
	cloudBottom := leadingSpanB
	if leadingSpanB > leadingSpanA {
		cloudTop = leadingSpanB
		cloudBottom = leadingSpanA
	}

	cloudTop52 := leadingSpanA52
	cloudBottom52 := leadingSpanB52
	if leadingSpanB52 > leadingSpanA52 {
		cloudTop52 = leadingSpanB52
		cloudBottom52 = leadingSpanA52
	}

	// ── 8. Signal conditions ──────────────────────────────────────────────────

	// Condition 1 — Bullish Gold Cross (Conversion Line crosses above Base Line)
	// "The most important short-term signal in Ichimoku"
	//tkCross := conversionLine > baseLine

	// Condition 2 — Price is above the Cloud
	// "Confirms the trend direction; above = bullish"
	//priceAboveCloud := currentPrice > cloudTop

	// Condition 3 — Lagging Span confirmation
	// "Current price (acting as Lagging Span) is above price from 26 sessions ago"
	//laggingSpanBullish := currentPrice > priceFrom26SessionsAgo && currentPrice > cloudTop52

	// Condition 4 — Bullish Cloud (Leading Span A above Leading Span B)
	// "Green cloud = bullish momentum; red cloud = bearish"
	//bullishCloud := leadingSpanA > leadingSpanB

	curr_above_cloud := currentPrice > cloudTop
	lag_above_cdlhigh := currentPrice > priceFrom26SessionsAgo
	cl_above_bl := conversionLine >= baseLine
	curr_above_cl := currentPrice > conversionLine
	lag_above_prev_ctop := currentPrice > cloudTop52
	condition12 := yesterdayClose < priceFrom27SessionsAgo

	// Condition 6 — Cloud cross-out filter (the new addition).
	// Fires only when price has broken ABOVE the cloud for the first time TODAY.
	// Two scenarios, either of which qualifies:
	//
	//   A) Intraday breakout: today's open was at or below the cloud top
	//      (price started the session inside or below the cloud), and the
	//      current tick is now above the cloud. The stock crossed out during
	//      the live session.
	//
	//   B) Gap-open breakout: yesterday's close was inside or below the cloud
	//      (price ended the previous session not yet above the cloud), and
	//      today's open gapped directly above it. The cross happened overnight.
	//
	// Without this filter, condition1 (currentPrice > cloudTop) would fire on
	// every subsequent tick for a stock that has been above the cloud for weeks.
	// condition6 ensures the alert is a fresh event, not a stale one.
	intradayBreakout := todayOpen <= cloudTop && currentPrice > cloudTop
	gapOpenBreakout := yesterdayClose <= cloudTop && todayOpen > cloudTop
	condition6 := intradayBreakout || gapOpenBreakout

	intradaySell := todayOpen >= cloudBottom && currentPrice < cloudBottom
	gapOpenSell := yesterdayClose >= cloudBottom && todayOpen < cloudBottom
	condition7 := intradaySell || gapOpenSell
	bl_above_cl := baseLine > conversionLine
	cl_above_curr := conversionLine > currentPrice
	lag_below_prev_cld_btm := cloudBottom52 > currentPrice
	lag_below_cdl := currentPrice < priceFrom26SessionsAgolow
	condition13 := yesterdayClose > priceFrom27SessionsAgolow
	curr_below_cld_btm := currentPrice < cloudBottom
	gold_cross := prevBaseLine >= prevConversionLine && baseLine < conversionLine
	lag_above_prevcl := currentPrice > pastConversionLine
	lag_cross := yesterdayClose < pastPrevConversionLine && currentPrice > pastConversionLine
	death_cross := prevConversionLine >= prevBaseLine && baseLine > conversionLine
	cl_above_cdl := conversionLine > currentPrice
	lag_below_cl := pastConversionLine > currentPrice
	lag_death_cross := yesterdayClose >= pastPrevConversionLine && currentPrice < pastConversionLine

	// ── 9. Full Ichimoku buy signal ───────────────────────────────────────────
	// Future BUY is the full buy plus a fresh break above last month's high and
	// price above the weekly Conversion Line. It is sent instead of the plain buy.
	buy := curr_above_cloud && lag_above_cdlhigh && cl_above_bl && curr_above_cl && lag_above_prev_ctop && condition6
	if buy && monthlyBreakUp(history, today, todayOpen, currentPrice) {
		if weeklyCL, ok := weeklyConversionLine(history, today, todayHigh, todayLow); ok && currentPrice > weeklyCL {
			return true, domain.OrderSignal{
				InstrumentToken: tick.InstrumentToken,
				TradingSymbol:   "",
				StrategyName:    i.Name() + " (Future BUY)",
				SignalTime:      time.Now().UTC(),
			}
		}
	}
	if buy {
		return true, domain.OrderSignal{
			InstrumentToken: tick.InstrumentToken,
			TradingSymbol:   "",
			StrategyName:    i.Name() + " (BUY)",
			SignalTime:      time.Now().UTC(),
		}
	}
	if curr_above_cloud && lag_above_cdlhigh && cl_above_bl && curr_above_cl && lag_above_prev_ctop && condition12 {
		return true, domain.OrderSignal{
			InstrumentToken: tick.InstrumentToken,
			TradingSymbol:   "",
			StrategyName:    i.Name() + " (BUY) 1A",
			SignalTime:      time.Now().UTC(),
		}

	}

	if gold_cross && lag_above_cdlhigh && lag_above_prevcl && curr_above_cl {

		return true, domain.OrderSignal{
			InstrumentToken: tick.InstrumentToken,
			TradingSymbol:   "",
			StrategyName:    i.Name() + " (BUY) Gold Cross",
			SignalTime:      time.Now().UTC(),
		}
	}

	if cl_above_bl && lag_above_cdlhigh && curr_above_cl && lag_cross {
		return true, domain.OrderSignal{
			InstrumentToken: tick.InstrumentToken,
			TradingSymbol:   "",
			StrategyName:    i.Name() + " (BUY) Gold Cross A",
			SignalTime:      time.Now().UTC(),
		}
	}

	// Future SELL mirrors Future BUY: the full sell plus a fresh break below last
	// month's low and price below the weekly Conversion Line.
	sell := condition7 && bl_above_cl && cl_above_curr && lag_below_prev_cld_btm && lag_below_cdl && curr_below_cld_btm
	if sell && monthlyBreakDown(history, today, todayOpen, currentPrice) {
		if weeklyCL, ok := weeklyConversionLine(history, today, todayHigh, todayLow); ok && currentPrice < weeklyCL {
			return true, domain.OrderSignal{
				InstrumentToken: tick.InstrumentToken,
				TradingSymbol:   "",
				StrategyName:    i.Name() + " (Future SELL)",
				SignalTime:      time.Now().UTC(),
			}
		}
	}
	if sell {
		return true, domain.OrderSignal{
			InstrumentToken: tick.InstrumentToken,
			TradingSymbol:   "",
			StrategyName:    i.Name() + " (SELL)",
			SignalTime:      time.Now().UTC(),
		}

	}

	if bl_above_cl && cl_above_curr && lag_below_prev_cld_btm && lag_below_cdl && curr_below_cld_btm && condition13 {
		return true, domain.OrderSignal{
			InstrumentToken: tick.InstrumentToken,
			TradingSymbol:   "",
			StrategyName:    i.Name() + " (SELL) 1A",
			SignalTime:      time.Now().UTC(),
		}
	}

	if death_cross && cl_above_cdl && lag_below_cdl && lag_below_cl {
		return true, domain.OrderSignal{
			InstrumentToken: tick.InstrumentToken,
			TradingSymbol:   "",
			StrategyName:    i.Name() + " (SELL) Death Cross",
			SignalTime:      time.Now().UTC(),
		}
	}

	if lag_death_cross && cl_above_cdl && bl_above_cl {
		return true, domain.OrderSignal{
			InstrumentToken: tick.InstrumentToken,
			TradingSymbol:   "",
			StrategyName:    i.Name() + " (SELL) Death Cross A",
			SignalTime:      time.Now().UTC(),
		}
	}

	return false, domain.OrderSignal{}
}

// monthLevels holds the previous calendar month's range and the range of the
// current month's sessions before today.
type monthLevels struct {
	prevHigh, prevLow   float64
	prevOK              bool // the previous month has candles
	soFarHigh, soFarLow float64
	soFarOK             bool // false on the month's first session
}

// monthLevelsBefore reads monthLevels from before, the sessions before today,
// oldest first. Months are IST calendar months.
func monthLevelsBefore(before []kiteconnect.HistoricalData, today time.Time) monthLevels {
	t := today.In(domain.IST)
	thisMonth := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, domain.IST)
	prevMonth := thisMonth.AddDate(0, -1, 0)

	var lv monthLevels
	for i := len(before) - 1; i >= 0; i-- {
		c := before[i]
		d := c.Date.Time.In(domain.IST)
		switch {
		case !d.Before(thisMonth):
			if !lv.soFarOK || c.High > lv.soFarHigh {
				lv.soFarHigh = c.High
			}
			if !lv.soFarOK || c.Low < lv.soFarLow {
				lv.soFarLow = c.Low
			}
			lv.soFarOK = true
		case !d.Before(prevMonth):
			if !lv.prevOK || c.High > lv.prevHigh {
				lv.prevHigh = c.High
			}
			if !lv.prevOK || c.Low < lv.prevLow {
				lv.prevLow = c.Low
			}
			lv.prevOK = true
		default:
			return lv
		}
	}
	return lv
}

// monthlyBreakUp reports whether today is the month's first break above the
// previous month's high: either the price crossed it during today's session
// (open at or below, now above) or today gapped over it (yesterday's close at
// or below, today's open above). No earlier session this month may have gone
// above it already.
func monthlyBreakUp(before []kiteconnect.HistoricalData, today time.Time, todayOpen, price float64) bool {
	lv := monthLevelsBefore(before, today)
	if !lv.prevOK || (lv.soFarOK && lv.soFarHigh > lv.prevHigh) {
		return false
	}
	level := lv.prevHigh
	yesterdayClose := before[len(before)-1].Close
	return (todayOpen <= level && price > level) || (yesterdayClose <= level && todayOpen > level)
}

// monthlyBreakDown mirrors monthlyBreakUp for the previous month's low.
func monthlyBreakDown(before []kiteconnect.HistoricalData, today time.Time, todayOpen, price float64) bool {
	lv := monthLevelsBefore(before, today)
	if !lv.prevOK || (lv.soFarOK && lv.soFarLow < lv.prevLow) {
		return false
	}
	level := lv.prevLow
	yesterdayClose := before[len(before)-1].Close
	return (todayOpen >= level && price < level) || (yesterdayClose >= level && todayOpen < level)
}

// weeklyConversionLine is the Conversion Line on the weekly chart: the midpoint
// of the highest high and lowest low over the last 9 weeks (IST ISO weeks),
// counting the current week, which includes today's high and low. ok is false
// when before holds fewer than 9 weeks.
func weeklyConversionLine(before []kiteconnect.HistoricalData, today time.Time, todayHigh, todayLow float64) (float64, bool) {
	const weeksNeeded = 9
	year, week := today.In(domain.IST).ISOWeek()
	high, low := todayHigh, todayLow
	weeks := 1
	for i := len(before) - 1; i >= 0; i-- {
		c := before[i]
		y, w := c.Date.Time.In(domain.IST).ISOWeek()
		if y != year || w != week {
			if weeks == weeksNeeded {
				break
			}
			weeks++
			year, week = y, w
		}
		if c.High > high {
			high = c.High
		}
		if c.Low < low {
			low = c.Low
		}
	}
	return (high + low) / 2.0, weeks == weeksNeeded
}
