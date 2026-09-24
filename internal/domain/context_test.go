package domain

import (
	"testing"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
	kitemodels "github.com/zerodha/gokiteconnect/v4/models"
)

func candle(y int, m time.Month, d int, close float64) kiteconnect.HistoricalData {
	return kiteconnect.HistoricalData{Date: kitemodels.Time{Time: time.Date(y, m, d, 0, 0, 0, 0, IST)}, Close: close}
}

func TestAppendReplacesSameDayCandle(t *testing.T) {
	mc := NewMarketContext()
	mc.Set(1, []kiteconnect.HistoricalData{candle(2026, 9, 23, 100), candle(2026, 9, 24, 105)}) // 24th is a partial
	mc.Append(1, candle(2026, 9, 24, 110))                                                      // the final candle for the 24th

	got, _ := mc.Get(1)
	if len(got) != 2 || got[1].Close != 110 {
		t.Fatalf("got %d candles, last close %v; want 2 with the 24th replaced (110)", len(got), got[len(got)-1].Close)
	}

	mc.Append(1, candle(2026, 9, 25, 120)) // a new day still appends
	if got, _ = mc.Get(1); len(got) != 3 || got[2].Close != 120 {
		t.Fatalf("new day: got %d candles", len(got))
	}
}
