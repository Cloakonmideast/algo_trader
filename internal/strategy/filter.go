package strategy

import (
	"strings"

	"github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
)

// SignalFilter wraps a Strategy and forwards only the signals whose names appear
// in an allowlist. Everything else is evaluated and then discarded.
//
// The point is alert volume. A strategy like RSI emits a dozen signal variants and
// fires tens of thousands of times a year across the universe; registering it
// directly would bury the handful of setups actually worth acting on. Wrapping it
// lets the engine subscribe to just those.
//
// The inner strategy is ALWAYS evaluated, never short-circuited, so a stateful
// strategy still observes every tick and cannot desync from price.
type SignalFilter struct {
	inner Strategy
	allow map[string]struct{}
}

// OnlySignals wraps inner so that only signals whose StrategyName exactly matches
// one of names is forwarded. Matching is case-insensitive.
//
// With no names given, nothing is forwarded — an explicit "subscribe to nothing"
// rather than an accidental firehose.
func OnlySignals(inner Strategy, names ...string) *SignalFilter {
	allow := make(map[string]struct{}, len(names))
	for _, n := range names {
		allow[strings.ToLower(strings.TrimSpace(n))] = struct{}{}
	}
	return &SignalFilter{inner: inner, allow: allow}
}

// Name returns the wrapped strategy's name, so logs and Discord alerts read as
// the underlying strategy rather than the wrapper.
func (f *SignalFilter) Name() string { return f.inner.Name() }

// Allows reports whether a signal name would be forwarded. Exposed for startup
// logging so a typo in the allowlist is visible at boot rather than as silence.
func (f *SignalFilter) Allows(signalName string) bool {
	_, ok := f.allow[strings.ToLower(strings.TrimSpace(signalName))]
	return ok
}

// Evaluate returns the first allowed signal the wrapped strategy fires.
func (f *SignalFilter) Evaluate(tick models.Tick, ctx *domain.MarketContext) (bool, domain.OrderSignal) {
	allowed := f.EvaluateAll(tick, ctx)
	if len(allowed) == 0 {
		return false, domain.OrderSignal{}
	}
	return true, allowed[0]
}

// EvaluateAll runs the wrapped strategy and keeps every signal whose name is
// allowed, preserving the inner strategy's priority order.
func (f *SignalFilter) EvaluateAll(tick models.Tick, ctx *domain.MarketContext) []domain.OrderSignal {
	var allowed []domain.OrderSignal
	for _, signal := range EvaluateAll(f.inner, tick, ctx) {
		if f.Allows(signal.StrategyName) {
			allowed = append(allowed, signal)
		}
	}
	return allowed
}
