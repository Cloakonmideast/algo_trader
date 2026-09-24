package service

import (
	"time"

	"algotrader/internal/domain"
)

// marketCloseMinute is 15:30 IST: before it, today's daily candle is unfinished.
const marketCloseMinute = 15*60 + 30

// calendarToken is the NIFTY 50 index. It has a daily candle for exactly the
// days NSE traded, so it serves as the trading calendar (holidays included)
// without a hand-maintained holiday list.
const calendarToken = 256265

// sessionProbe returns the days in [from, to] on which the market traded.
type sessionProbe func(from, to time.Time) ([]time.Time, error)

func dayStart(t time.Time) time.Time {
	t = t.In(domain.IST)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, domain.IST)
}

func dayEnd(t time.Time) time.Time { return dayStart(t).Add(24*time.Hour - time.Minute) }

func isWeekend(t time.Time) bool {
	wd := t.In(domain.IST).Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

// lastCompletedWeekday is the latest weekday whose daily candle would be final:
// today once the market has closed, otherwise the previous day, stepping back
// over weekends.
func lastCompletedWeekday(now time.Time) time.Time {
	n := now.In(domain.IST)
	day := dayStart(n)
	if n.Hour()*60+n.Minute() < marketCloseMinute {
		day = day.AddDate(0, 0, -1)
	}
	for isWeekend(day) {
		day = day.AddDate(0, 0, -1)
	}
	return day
}

// lastTradingSession is the latest day the market actually traded and closed,
// read from the index calendar so holidays are skipped. If the calendar can't
// be read it returns the last completed weekday together with the error; the
// usual cause is no valid Kite login yet, in which case every request would fail.
func lastTradingSession(now time.Time, probe sessionProbe) (time.Time, error) {
	last := lastCompletedWeekday(now)
	sessions, err := probe(last.AddDate(0, 0, -14), dayEnd(last))
	if err != nil {
		return last, err
	}
	if len(sessions) == 0 {
		return last, nil
	}
	var latest time.Time
	for _, s := range sessions {
		if d := dayStart(s); !d.After(last) && d.After(latest) {
			latest = d
		}
	}
	if latest.IsZero() {
		return last, nil
	}
	return latest, nil
}

// gapWindow is the range to fetch so that stored history, whose newest candle
// is latest, reaches lastSession. need is false when nothing is missing, so a
// stock that already has the last trading session costs no request.
func gapWindow(latest, lastSession time.Time) (from, to time.Time, need bool) {
	if !dayStart(latest).Before(dayStart(lastSession)) {
		return time.Time{}, time.Time{}, false
	}
	return dayStart(latest).AddDate(0, 0, 1), dayEnd(lastSession), true
}

// isTradingDay reports whether the market traded on t's day. Weekends are
// answered without a request; if the calendar can't be read on a weekday, it
// assumes the market traded so the 15:45 rollup still tries.
func isTradingDay(t time.Time, probe sessionProbe) bool {
	if isWeekend(t) {
		return false
	}
	sessions, err := probe(dayStart(t), dayEnd(t))
	if err != nil {
		return true
	}
	for _, s := range sessions {
		if dayStart(s).Equal(dayStart(t)) {
			return true
		}
	}
	return false
}
