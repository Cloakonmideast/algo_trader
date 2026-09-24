package service

import "sync/atomic"

// alertGate lets each WebSocket raise one Discord alert per outage. Without it,
// every reconnect attempt against a rejected or expired token posts an alert,
// which floods the channel for hours on a long-running engine.
type alertGate struct {
	alerted [numWebSockets]atomic.Bool
}

// shouldAlert reports whether connection idx should alert now (the first error
// since it was last connected).
func (g *alertGate) shouldAlert(idx int) bool {
	return g.alerted[idx].CompareAndSwap(false, true)
}

// reset re-arms alerts for connection idx after it connects successfully.
func (g *alertGate) reset(idx int) {
	g.alerted[idx].Store(false)
}
