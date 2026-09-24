package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
)

type fakeSender struct {
	mu   sync.Mutex
	sent []domain.OrderSignal
}

func (f *fakeSender) SendSignal(s domain.OrderSignal) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, s)
	return nil
}

// fakeSaver answers InsertSignal from a script: each call pops the next result.
type fakeSaver struct {
	mu      sync.Mutex
	results []saveResult
	calls   []domain.OrderSignal
}

type saveResult struct {
	inserted bool
	err      error
}

func (f *fakeSaver) InsertSignal(_ context.Context, s domain.OrderSignal) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, s)
	r := f.results[0]
	if len(f.results) > 1 {
		f.results = f.results[1:]
	}
	return r.inserted, r.err
}

func newTestProcessor(saver SignalSaver, sender *fakeSender) *Processor {
	p := NewProcessor(10, 1, domain.NewStateRegistry(), domain.NewMarketContext(), sender, nil)
	if saver != nil {
		p.UseStore(saver)
	}
	return p
}

func TestEnrichFillsFamilyPriceAndDirection(t *testing.T) {
	s := enrich(domain.OrderSignal{StrategyName: "IchimokuKinkoHyo (SELL) 1A"}, "IchimokuKinkoHyo", models.Tick{LastPrice: 812.5})
	if s.Strategy != "IchimokuKinkoHyo" || s.Price != 812.5 || s.Direction != domain.Short {
		t.Errorf("enrich = %+v", s)
	}
}

func TestEnrichKeepsValuesTheStrategySet(t *testing.T) {
	in := domain.OrderSignal{StrategyName: "RSI (GFS)", Strategy: "Custom", Price: 100}
	s := enrich(in, "RSI", models.Tick{LastPrice: 999})
	if s.Strategy != "Custom" || s.Price != 100 || s.Direction != domain.Long {
		t.Errorf("enrich = %+v", s)
	}
}

func TestDispatchSavesThenAlerts(t *testing.T) {
	saver := &fakeSaver{results: []saveResult{{inserted: true}}}
	sender := &fakeSender{}
	newTestProcessor(saver, sender).dispatchSignal(domain.OrderSignal{StrategyName: "RSI (GFS)"})
	if len(saver.calls) != 1 || len(sender.sent) != 1 {
		t.Errorf("saves=%d alerts=%d, want 1 and 1", len(saver.calls), len(sender.sent))
	}
}

func TestDispatchSkipsAlertWhenAlreadySavedToday(t *testing.T) {
	sender := &fakeSender{}
	newTestProcessor(&fakeSaver{results: []saveResult{{inserted: false}}}, sender).dispatchSignal(domain.OrderSignal{StrategyName: "RSI (GFS)"})
	if len(sender.sent) != 0 {
		t.Errorf("a signal already in Supabase must not re-alert after a restart; alerts=%d", len(sender.sent))
	}
}

func TestDispatchAlertsAndQueuesWhenSaveFails(t *testing.T) {
	saver := &fakeSaver{results: []saveResult{{err: errors.New("supabase down")}}}
	sender := &fakeSender{}
	p := newTestProcessor(saver, sender)
	p.dispatchSignal(domain.OrderSignal{StrategyName: "RSI (GFS)"})
	if len(sender.sent) != 1 {
		t.Errorf("Discord must still alert when Supabase is down; alerts=%d", len(sender.sent))
	}
	if n := p.pending.len(); n != 1 {
		t.Errorf("failed save should be queued for retry; queued=%d", n)
	}
}

func TestDispatchWithoutStoreOnlyAlerts(t *testing.T) {
	sender := &fakeSender{}
	newTestProcessor(nil, sender).dispatchSignal(domain.OrderSignal{StrategyName: "RSI (GFS)"})
	if len(sender.sent) != 1 {
		t.Errorf("alerts=%d, want 1", len(sender.sent))
	}
}

// firingStrategy fires one bare signal on every tick, like Ichimoku does.
type firingStrategy struct{}

func (firingStrategy) Name() string { return "FakeFamily" }
func (firingStrategy) Evaluate(tick models.Tick, _ *domain.MarketContext) (bool, domain.OrderSignal) {
	return true, domain.OrderSignal{InstrumentToken: tick.InstrumentToken, StrategyName: "FakeFamily (SELL)"}
}

func TestProcessTickEnrichesSavedSignals(t *testing.T) {
	saver := &fakeSaver{results: []saveResult{{inserted: true}}}
	sender := &fakeSender{}
	p := NewProcessor(10, 1, domain.NewStateRegistry(), domain.NewMarketContext(), sender, map[uint32]string{7: "TCS"})
	p.UseStore(saver)
	p.RegisterStrategy(firingStrategy{})

	p.processTick(models.Tick{InstrumentToken: 7, LastPrice: 50.25})

	deadline := time.Now().Add(2 * time.Second)
	for {
		saver.mu.Lock()
		n := len(saver.calls)
		saver.mu.Unlock()
		if n == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	saver.mu.Lock()
	defer saver.mu.Unlock()
	if len(saver.calls) != 1 {
		t.Fatalf("saves = %d, want 1", len(saver.calls))
	}
	s := saver.calls[0]
	if s.Strategy != "FakeFamily" || s.Price != 50.25 || s.Direction != domain.Short || s.TradingSymbol != "TCS" {
		t.Errorf("saved signal = %+v", s)
	}
}

func TestSaveQueueFlush(t *testing.T) {
	q := newSaveQueue(10)
	for _, name := range []string{"a", "b", "c"} {
		q.add(domain.OrderSignal{StrategyName: name})
	}
	// a saves, b fails again, c turns out to be a duplicate (already saved): only b stays queued.
	saver := &fakeSaver{results: []saveResult{{inserted: true}, {err: errors.New("still down")}, {inserted: false}}}
	saved, remaining := q.flush(context.Background(), saver)
	if saved != 2 || remaining != 1 {
		t.Errorf("flush: saved=%d remaining=%d, want 2 and 1", saved, remaining)
	}
	if q.items[0].StrategyName != "b" {
		t.Errorf("remaining item = %q, want b", q.items[0].StrategyName)
	}
}

func TestSaveQueueIsBounded(t *testing.T) {
	q := newSaveQueue(2)
	q.add(domain.OrderSignal{StrategyName: "a"})
	q.add(domain.OrderSignal{StrategyName: "b"})
	if q.add(domain.OrderSignal{StrategyName: "c"}) {
		t.Error("a full queue must refuse new items")
	}
	if q.len() != 2 {
		t.Errorf("len = %d, want 2", q.len())
	}
}
