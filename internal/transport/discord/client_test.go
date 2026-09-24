package discord

import (
	"testing"
	"time"

	"algotrader/internal/domain"
)

func TestSignalEmbedShowsSideAndPrice(t *testing.T) {
	short := signalEmbed(domain.OrderSignal{
		TradingSymbol: "TCS", StrategyName: "RSI (Bearish GFS)", Direction: domain.Short,
		Price: 3999.5, SignalTime: time.Date(2026, 9, 25, 4, 0, 0, 0, time.UTC),
	})
	if short.Color != colorRed {
		t.Errorf("short signal color = %#x, want red", short.Color)
	}
	if want := "**TCS** triggered **RSI (Bearish GFS)** · SHORT @ ₹3999.50"; short.Description != want {
		t.Errorf("description = %q, want %q", short.Description, want)
	}

	long := signalEmbed(domain.OrderSignal{TradingSymbol: "INFY", StrategyName: "RSI (GFS)", Direction: domain.Long})
	if long.Color != colorGreen {
		t.Errorf("long signal color = %#x, want green", long.Color)
	}
	if want := "**INFY** triggered **RSI (GFS)** · LONG"; long.Description != want {
		t.Errorf("description without a price = %q, want %q", long.Description, want)
	}
}
