package strategy

import (
	"strings"
	"testing"

	"github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
)

// countingStrategy records how often it was evaluated and emits a fixed signal,
// letting the tests assert both the filtering and the pass-through of every tick.
type countingStrategy struct {
	calls  int
	fire   bool
	signal string
}

func (c *countingStrategy) Name() string { return "Counter" }

func (c *countingStrategy) Evaluate(models.Tick, *domain.MarketContext) (bool, domain.OrderSignal) {
	c.calls++
	if !c.fire {
		return false, domain.OrderSignal{}
	}
	return true, domain.OrderSignal{StrategyName: c.signal}
}

func TestSignalFilterForwardsAllowedSignal(t *testing.T) {
	inner := &countingStrategy{fire: true, signal: "RSI (GFS)"}
	f := OnlySignals(inner, "RSI (GFS)", "RSI (Advanced GFS)")

	fired, sig := f.Evaluate(models.Tick{}, domain.NewMarketContext())
	if !fired {
		t.Fatal("an allowlisted signal must be forwarded")
	}
	if sig.StrategyName != "RSI (GFS)" {
		t.Errorf("signal name altered: got %q", sig.StrategyName)
	}
}

func TestSignalFilterBlocksOtherSignals(t *testing.T) {
	inner := &countingStrategy{fire: true, signal: "RSI (Support)"}
	f := OnlySignals(inner, "RSI (GFS)", "RSI (Advanced GFS)")

	if fired, _ := f.Evaluate(models.Tick{}, domain.NewMarketContext()); fired {
		t.Error("a signal outside the allowlist must not be forwarded")
	}
}

// The whole point of the wrapper is that it filters the OUTPUT, never the input:
// RSI's range-shift machines must observe every tick or they desync from price.
func TestSignalFilterAlwaysEvaluatesInner(t *testing.T) {
	inner := &countingStrategy{fire: true, signal: "RSI (Support)"} // always blocked
	f := OnlySignals(inner, "RSI (GFS)")

	ctx := domain.NewMarketContext()
	for i := 0; i < 5; i++ {
		f.Evaluate(models.Tick{}, ctx)
	}

	if inner.calls != 5 {
		t.Errorf("inner strategy must run on every tick even when filtered out: got %d of 5", inner.calls)
	}
}

func TestSignalFilterMatchingIsCaseInsensitive(t *testing.T) {
	inner := &countingStrategy{fire: true, signal: "RSI (Advanced GFS)"}
	f := OnlySignals(inner, "rsi (advanced gfs)")

	if fired, _ := f.Evaluate(models.Tick{}, domain.NewMarketContext()); !fired {
		t.Error("allowlist matching should ignore case")
	}
}

// An empty allowlist is an explicit "subscribe to nothing", not a firehose.
func TestSignalFilterEmptyAllowlistBlocksEverything(t *testing.T) {
	inner := &countingStrategy{fire: true, signal: "RSI (GFS)"}
	f := OnlySignals(inner)

	if fired, _ := f.Evaluate(models.Tick{}, domain.NewMarketContext()); fired {
		t.Error("an empty allowlist must forward nothing")
	}
	if inner.calls != 1 {
		t.Error("inner strategy must still be evaluated with an empty allowlist")
	}
}

// multiStrategy fires several signals on the same tick, like RSI does when a
// weekly and a daily range-shift land on the same day.
type multiStrategy struct{ signals []string }

func (m *multiStrategy) Name() string { return "Multi" }

func (m *multiStrategy) Evaluate(tick models.Tick, ctx *domain.MarketContext) (bool, domain.OrderSignal) {
	all := m.EvaluateAll(tick, ctx)
	if len(all) == 0 {
		return false, domain.OrderSignal{}
	}
	return true, all[0]
}

func (m *multiStrategy) EvaluateAll(models.Tick, *domain.MarketContext) []domain.OrderSignal {
	out := make([]domain.OrderSignal, 0, len(m.signals))
	for _, s := range m.signals {
		out = append(out, domain.OrderSignal{StrategyName: s})
	}
	return out
}

func names(signals []domain.OrderSignal) []string {
	out := make([]string, 0, len(signals))
	for _, s := range signals {
		out = append(out, s.StrategyName)
	}
	return out
}

func TestEvaluateAllWrapsSingleSignalStrategies(t *testing.T) {
	got := EvaluateAll(&countingStrategy{fire: true, signal: "X"}, models.Tick{}, domain.NewMarketContext())
	if len(got) != 1 || got[0].StrategyName != "X" {
		t.Errorf("expected the single signal, got %v", names(got))
	}
	if got := EvaluateAll(&countingStrategy{}, models.Tick{}, domain.NewMarketContext()); len(got) != 0 {
		t.Errorf("expected no signals, got %v", names(got))
	}
}

func TestSignalFilterForwardsEveryAllowedSignal(t *testing.T) {
	inner := &multiStrategy{signals: []string{"A", "B", "C"}}
	f := OnlySignals(inner, "A", "C")

	got := EvaluateAll(f, models.Tick{}, domain.NewMarketContext())
	if strings.Join(names(got), ",") != "A,C" {
		t.Errorf("expected A and C, got %v", names(got))
	}

	// Evaluate still reports the first allowed signal for single-signal callers.
	f = OnlySignals(inner, "B")
	if fired, sig := f.Evaluate(models.Tick{}, domain.NewMarketContext()); !fired || sig.StrategyName != "B" {
		t.Errorf("Evaluate should return the first allowed signal, got fired=%v %q", fired, sig.StrategyName)
	}
}

// The wrapper must report the inner name so logs and Discord alerts read as the
// real strategy.
func TestSignalFilterReportsInnerName(t *testing.T) {
	f := OnlySignals(&countingStrategy{}, "anything")
	if f.Name() != "Counter" {
		t.Errorf("Name() should delegate to the wrapped strategy, got %q", f.Name())
	}
}
