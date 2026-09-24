package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
)

// SignalSaver persists a signal for the website. InsertSignal reports
// inserted=false when the same signal was already saved for that stock today.
// Implemented by *supabase.Client.
type SignalSaver interface {
	InsertSignal(ctx context.Context, s domain.OrderSignal) (inserted bool, err error)
}

// enrich fills the fields strategies usually leave empty: the strategy family
// (the registered strategy's Name()), the price from the tick, and the side
// derived from the signal name. Values a strategy did set are kept.
func enrich(s domain.OrderSignal, family string, tick models.Tick) domain.OrderSignal {
	if s.Strategy == "" {
		s.Strategy = family
	}
	if s.Price == 0 {
		s.Price = tick.LastPrice
	}
	s.Direction = domain.DirectionOf(s.StrategyName)
	return s
}

// saveQueue holds signals whose save failed (Supabase unreachable) so they can
// be retried. It is bounded so a long outage cannot grow memory without limit.
type saveQueue struct {
	mu    sync.Mutex
	items []domain.OrderSignal
	cap   int
}

func newSaveQueue(capacity int) *saveQueue {
	return &saveQueue{cap: capacity}
}

// add queues a signal, returning false (and dropping it) when the queue is full.
func (q *saveQueue) add(s domain.OrderSignal) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) >= q.cap {
		return false
	}
	q.items = append(q.items, s)
	return true
}

func (q *saveQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// flush retries every queued signal once. Saved rows and duplicates (already in
// Supabase) leave the queue; failures stay for the next flush.
func (q *saveQueue) flush(ctx context.Context, saver SignalSaver) (saved, remaining int) {
	q.mu.Lock()
	batch := q.items
	q.items = nil
	q.mu.Unlock()

	var failed []domain.OrderSignal
	for _, s := range batch {
		if _, err := saver.InsertSignal(ctx, s); err != nil {
			failed = append(failed, s)
			continue
		}
		saved++
	}

	q.mu.Lock()
	q.items = append(failed, q.items...)
	remaining = len(q.items)
	q.mu.Unlock()
	return saved, remaining
}

// runRetries flushes the queue every interval until ctx is cancelled.
func (q *saveQueue) runRetries(ctx context.Context, saver SignalSaver, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if q.len() == 0 {
				continue
			}
			saved, remaining := q.flush(ctx, saver)
			log.Printf("[processor] Retried queued signal saves: %d saved, %d still queued", saved, remaining)
		}
	}
}
