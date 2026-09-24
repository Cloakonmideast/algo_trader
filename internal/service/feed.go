package service

import "sync"

// Ticker is a started live feed; *MultiTicker implements it.
type Ticker interface {
	Start()
	Stop()
	Connected() int
}

// LiveFeed owns the current live feed and swaps it when the Kite token
// changes: a WebSocket authenticates once, so a new token needs new connections.
type LiveFeed struct {
	mu   sync.Mutex
	make func(token string) Ticker
	cur  Ticker
}

func NewLiveFeed(make func(token string) Ticker) *LiveFeed {
	return &LiveFeed{make: make}
}

// Start connects with token, closing any feed that was running.
func (f *LiveFeed) Start(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cur != nil {
		f.cur.Stop()
	}
	f.cur = f.make(token)
	f.cur.Start()
}

// Stop closes the feed if one is running.
func (f *LiveFeed) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cur != nil {
		f.cur.Stop()
		f.cur = nil
	}
}

// Connected reports whether at least one WebSocket is connected.
func (f *LiveFeed) Connected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cur != nil && f.cur.Connected() > 0
}
