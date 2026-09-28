package backtest

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	kitemodels "github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
	"algotrader/internal/repository"
)

// The backtester reads the engine's database, where sessions are stored as UTC
// timestamps (IST midnight = 18:30 UTC the day before). Its candles must come
// back on the IST calendar so reports show the session's own date.
func TestLoadCandlesReturnsISTDates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candles.db")
	repo, err := repository.NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	monday := time.Date(2026, time.September, 21, 0, 0, 0, 0, domain.IST)
	if err := repo.SaveCandles(1, []kiteconnect.HistoricalData{{Date: kitemodels.Time{Time: monday}, Close: 1}}); err != nil {
		t.Fatal(err)
	}
	repo.Close()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	got, err := loadCandles(db, 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("loadCandles = %d candles, err %v; want 1", len(got), err)
	}
	if d := got[0].Date.Time; !d.Equal(monday) || d.Format("2006-01-02 Mon") != "2026-09-21 Mon" {
		t.Errorf("date = %v, want 2026-09-21 Mon at IST midnight", d)
	}
}
