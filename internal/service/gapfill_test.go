package service

import (
	"errors"
	"testing"
	"time"
)

func istAt(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, time.FixedZone("IST", 5*3600+30*60))
}

func ymd(t time.Time) string { return t.Format("2006-01-02") }

func TestLastCompletedWeekday(t *testing.T) {
	cases := []struct {
		now  time.Time
		want string
	}{
		{istAt(2026, 9, 24, 9, 30), "2026-09-23"},  // Thu, market open: Wednesday
		{istAt(2026, 9, 24, 15, 29), "2026-09-23"}, // one minute before close
		{istAt(2026, 9, 24, 15, 30), "2026-09-24"}, // closed: today is final
		{istAt(2026, 9, 25, 0, 30), "2026-09-24"},  // just after midnight
		{istAt(2026, 9, 26, 11, 0), "2026-09-25"},  // Saturday → Friday
		{istAt(2026, 9, 27, 11, 0), "2026-09-25"},  // Sunday → Friday
		{istAt(2026, 9, 28, 9, 15), "2026-09-25"},  // Monday before close → Friday
		{istAt(2026, 9, 28, 16, 0), "2026-09-28"},  // Monday after close
	}
	for _, tc := range cases {
		if got := ymd(lastCompletedWeekday(tc.now)); got != tc.want {
			t.Errorf("lastCompletedWeekday(%s) = %s, want %s", tc.now.Format("Mon 2006-01-02 15:04"), got, tc.want)
		}
	}
}

func TestLastTradingSessionUsesTheIndexCalendar(t *testing.T) {
	// Thursday 2 Oct is a holiday: the index has no candle for it.
	probe := func(from, to time.Time) ([]time.Time, error) {
		if ymd(to) != "2026-10-02" {
			t.Errorf("probe asked up to %s, want the last completed weekday 2026-10-02", ymd(to))
		}
		return []time.Time{istAt(2026, 9, 30, 0, 0), istAt(2026, 10, 1, 0, 0)}, nil
	}
	got, err := lastTradingSession(istAt(2026, 10, 3, 10, 0), probe) // Saturday
	if err != nil || ymd(got) != "2026-10-01" {
		t.Errorf("last trading session = %s, want 2026-10-01 (the 2nd was a holiday)", ymd(got))
	}
}

func TestLastTradingSessionFallsBackToWeekdays(t *testing.T) {
	failing := func(from, to time.Time) ([]time.Time, error) { return nil, errors.New("token expired") }
	got, err := lastTradingSession(istAt(2026, 9, 27, 11, 0), failing)
	if err == nil || ymd(got) != "2026-09-25" {
		t.Errorf("fallback = %s, err=%v; want Friday 2026-09-25 and the error (so Boot can skip gap fills)", ymd(got), err)
	}
	empty := func(from, to time.Time) ([]time.Time, error) { return nil, nil }
	if got, err := lastTradingSession(istAt(2026, 9, 27, 11, 0), empty); err != nil || ymd(got) != "2026-09-25" {
		t.Errorf("empty probe = %s, want the weekday fallback", ymd(got))
	}
}

func TestGapWindow(t *testing.T) {
	mon := istAt(2026, 9, 21, 0, 0)
	day := func(t time.Time) string { return t.Format("2006-01-02 15:04") }

	// Tuesday missed: fill Tuesday only.
	from, to, need := gapWindow(mon, istAt(2026, 9, 22, 0, 0))
	if !need || day(from) != "2026-09-22 00:00" || day(to) != "2026-09-22 23:59" {
		t.Errorf("need=%v from=%s to=%s, want Tuesday only", need, day(from), day(to))
	}

	// Stored up to the last trading session (e.g. Friday, checked on a Sunday
	// or a Monday morning): no request at all.
	fri := istAt(2026, 9, 25, 0, 0)
	if _, _, need = gapWindow(fri, fri); need {
		t.Error("nothing to fetch when the stock already has the last trading session")
	}
}

func TestIsTradingDay(t *testing.T) {
	sessions := []time.Time{istAt(2026, 10, 1, 0, 0)}
	probe := func(from, to time.Time) ([]time.Time, error) { return sessions, nil }
	if !isTradingDay(istAt(2026, 10, 1, 15, 45), probe) {
		t.Error("1 Oct traded")
	}
	sessions = nil
	if isTradingDay(istAt(2026, 10, 2, 15, 45), probe) {
		t.Error("2 Oct is a holiday: no index candle")
	}
	if isTradingDay(istAt(2026, 10, 3, 15, 45), probe) {
		t.Error("Saturday is never a trading day, without even asking")
	}
	failing := func(from, to time.Time) ([]time.Time, error) { return nil, errors.New("down") }
	if !isTradingDay(istAt(2026, 10, 1, 15, 45), failing) {
		t.Error("if the calendar can't be read on a weekday, assume it traded (the rollup then tries)")
	}
}
