package repository

import (
	"path/filepath"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	kitemodels "github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
)

// recentMonday is an IST-midnight Monday a week or more ago, inside the
// retention window whenever the test runs.
func recentMonday() time.Time {
	d := time.Now().In(domain.IST).AddDate(0, 0, -7)
	for d.Weekday() != time.Monday {
		d = d.AddDate(0, 0, -1)
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, domain.IST)
}

// Candles are stored as UTC timestamps, so an IST-midnight session is saved as
// 18:30 UTC the evening before. LoadHistory must hand them back on the IST
// calendar, or anything that reads a candle's day, week or month gets the
// previous day.
func TestLoadHistoryReturnsISTDates(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "candles.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	monday := recentMonday()
	if err := db.SaveCandles(1, []kiteconnect.HistoricalData{{Date: kitemodels.Time{Time: monday}, Open: 1, High: 1, Low: 1, Close: 1}}); err != nil {
		t.Fatal(err)
	}

	got, err := db.LoadHistory(1)
	if err != nil || len(got) != 1 {
		t.Fatalf("LoadHistory = %d candles, err %v; want 1", len(got), err)
	}
	d := got[0].Date.Time
	if !d.Equal(monday) || d.Weekday() != time.Monday || d.Hour() != 0 {
		t.Errorf("date = %v (%v); want %v, a Monday at midnight", d, d.Weekday(), monday)
	}
}
