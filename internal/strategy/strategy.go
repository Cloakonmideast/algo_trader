package strategy

import (
	"github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
)

// Strategy is the unified interface that all trading strategy implementations must satisfy.
// The engine evaluates each registered Strategy on every incoming market tick.
//
// Contract:
//   - Name() must return a stable, human-readable identifier (used in logs and Discord alerts).
//   - Evaluate() must be stateless with respect to shared mutable state; it receives a deep copy
//     of historical candles via MarketContext.Get() and must not retain references across calls.
//   - Evaluate() must be safe for concurrent invocation from multiple worker goroutines.
//   - Evaluate() returns (false, zero-value) when no entry signal is detected.
//   - Evaluate() returns (true, populated OrderSignal) when an entry condition is met.
type Strategy interface {
	// Name returns the human-readable strategy identifier used in logs and notifications.
	Name() string

	// Evaluate analyses the incoming tick against the strategy's decision rules using the
	// historical market data available in the MarketContext RAM cache.
	// Returns (true, signal) if an entry condition is triggered, (false, zero) otherwise.
	Evaluate(tick models.Tick, context *domain.MarketContext) (bool, domain.OrderSignal)
}

// MultiSignal is implemented by strategies that can fire more than one signal on
// the same tick — RSI, when a weekly and a daily range-shift land on the same
// day. Evaluate on such a strategy returns only the first (highest-priority) one.
type MultiSignal interface {
	EvaluateAll(tick models.Tick, context *domain.MarketContext) []domain.OrderSignal
}

// EvaluateAll returns every signal s fires on this tick, in priority order. It
// is what callers should use instead of Evaluate, so that multi-signal
// strategies never have their lower-priority signals silently dropped.
func EvaluateAll(s Strategy, tick models.Tick, context *domain.MarketContext) []domain.OrderSignal {
	if m, ok := s.(MultiSignal); ok {
		return m.EvaluateAll(tick, context)
	}
	if fired, signal := s.Evaluate(tick, context); fired {
		return []domain.OrderSignal{signal}
	}
	return nil
}
