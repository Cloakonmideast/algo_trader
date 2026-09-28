package service

import (
	"context"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/zerodha/gokiteconnect/v4/models"

	"algotrader/internal/domain"
	"algotrader/internal/strategy"
	"algotrader/internal/transport/discord"
)

// Processor is the concurrent tick evaluation engine.
// It manages a fixed-size worker goroutine pool that drains a high-throughput tick channel,
// applies registered strategies to each tick, and routes confirmed signals to Discord.
//
// Architecture:
//   - A single buffered input channel (chan models.Tick, cap 50,000) receives ticks from
//     the Kite WebSocket feed.
//   - A configurable pool of worker goroutines (default: 50) concurrently drain the channel.
//   - A StateRegistry prevents duplicate signals for the same instrument across workers.
//   - Each registered Strategy is evaluated per tick; confirmed signals are sent to Discord.
type Processor struct {
	tickCh     chan models.Tick
	strategies []strategy.Strategy
	registry   *domain.StateRegistry
	mktCtx     *domain.MarketContext
	discord    discord.SignalSender // accepts *discord.Client or *discord.BotClient
	workers    int
	wg         sync.WaitGroup
	// symbolMap provides token → trading symbol resolution for alert formatting.
	// Written once at construction; safe for concurrent reads without a lock.
	symbolMap map[uint32]string

	// store saves signals for the website (nil = Discord only); pending holds
	// saves that failed and are retried.
	store   SignalSaver
	pending *saveQueue

	// clock is time.Now outside tests.
	clock func() time.Time
}

// maxQueuedSaves bounds the retry queue during a long Supabase outage.
const maxQueuedSaves = 10_000

// NewProcessor constructs a Processor with the given tick channel capacity and worker pool size.
// Strategies can be registered after construction via RegisterStrategy.
func NewProcessor(
	bufferCapacity int,
	workerCount int,
	registry *domain.StateRegistry,
	mktCtx *domain.MarketContext,
	discordClient discord.SignalSender,
	symbolMap map[uint32]string,
) *Processor {
	if symbolMap == nil {
		symbolMap = make(map[uint32]string)
	}
	return &Processor{
		tickCh:    make(chan models.Tick, bufferCapacity),
		registry:  registry,
		mktCtx:    mktCtx,
		discord:   discordClient,
		workers:   workerCount,
		symbolMap: symbolMap,
		clock:     time.Now,
	}
}

// RegisterStrategy appends a strategy to the evaluation pipeline.
// Strategies are evaluated in registration order on every tick.
// This method must be called before Start(); calling it after Start() is a data race.
func (p *Processor) RegisterStrategy(s strategy.Strategy) {
	p.strategies = append(p.strategies, s)
	log.Printf("[processor] Registered strategy: %s", s.Name())
}

// Start launches the worker goroutine pool. Each worker independently drains the tick
// channel and evaluates all registered strategies. Workers run until the tick channel
// is closed, at which point they exit cleanly and the WaitGroup is decremented.
func (p *Processor) Start() {
	log.Printf("[processor] Starting %d worker goroutines (tick channel capacity: %d)", p.workers, cap(p.tickCh))
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.runWorker(i)
	}
}

// Stop closes the tick channel, signaling all workers to drain remaining ticks and exit.
// It blocks until every worker goroutine has returned, ensuring clean shutdown.
func (p *Processor) Stop() {
	log.Println("[processor] Stopping: closing tick channel and waiting for workers to drain")
	close(p.tickCh)
	p.wg.Wait()
	log.Println("[processor] All workers exited cleanly")
}

// Enqueue submits a tick to the processing channel in a non-blocking fashion.
// If the channel is at capacity (back-pressure scenario), the tick is dropped and
// a warning is logged to prevent the WebSocket feed goroutine from stalling.
func (p *Processor) Enqueue(tick models.Tick) {
	select {
	case p.tickCh <- tick:
	default:
		log.Printf("[processor] WARN: tick channel full (cap=%d), dropping tick for token %d",
			cap(p.tickCh), tick.InstrumentToken)
	}
}

// TickChannel exposes the write end of the tick channel so external feed adapters
// (e.g., the Kite WebSocket OnTick callback) can submit ticks directly.
func (p *Processor) TickChannel() chan<- models.Tick {
	return p.tickCh
}

// runWorker is the per-goroutine event loop. It drains the tick channel until it is
// closed, evaluating all registered strategies for each received tick.
func (p *Processor) runWorker(id int) {
	defer p.wg.Done()
	log.Printf("[processor] Worker %d started", id)

	for tick := range p.tickCh {
		p.processTick(tick)
	}

	log.Printf("[processor] Worker %d exiting — tick channel closed", id)
}

// processTick evaluates all registered strategies for the given tick.
// It first updates the IntradayCache with the latest price so that strategies
// always see a current today-OHLC snapshot via ctx.Intraday.
// It enforces the StateRegistry lock to ensure only the first worker to detect a
// breakout for a given instrument token executes the alert pipeline.
func (p *Processor) processTick(tick models.Tick) {
	// Only ticks from today's regular session are evaluated; see inLiveSession.
	now := p.clock().In(domain.IST)
	if !inLiveSession(now, tick.LastTradeTime.Time) {
		return
	}

	// The intraday day boundary is the IST calendar day, whatever the server's
	// own timezone (UTC on the cloud server).
	p.mktCtx.Intraday.UpdateAndGet(tick.InstrumentToken, tick.LastPrice, now)

	for _, strat := range p.strategies {
		// Evaluate the strategy inside a recover so a panic in any strategy
		// implementation does not kill the worker goroutine permanently.
		var signals []domain.OrderSignal
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[processor] PANIC in strategy %s for token %d: %v\n%s",
						strat.Name(), tick.InstrumentToken, r, debug.Stack())
				}
			}()
			signals = strategy.EvaluateAll(strat, tick, p.mktCtx)
		}()

		for _, signal := range signals {
			signal = enrich(signal, strat.Name(), tick)

			// Resolve TradingSymbol from the lookup map so Discord alerts show the
			// instrument name (e.g. "RELIANCE") instead of an empty string.
			if signal.TradingSymbol == "" {
				if sym, ok := p.symbolMap[tick.InstrumentToken]; ok {
					signal.TradingSymbol = sym
				} else {
					signal.TradingSymbol = "UNKNOWN"
				}
			}

			// Acquire this signal's daily alert slot for the token. Scoping by
			// signal name lets a weekly and a daily signal on the same stock both
			// alert, while a signal that keeps re-firing alerts only once a day.
			if !p.registry.TryLock(signal.StrategyName, tick.InstrumentToken) {
				continue
			}

			// We hold the lock — execute the alert pipeline asynchronously so we
			// don't block the worker from processing subsequent ticks.
			go p.dispatchSignal(signal)
		}
	}
}

// UseStore makes the processor save every signal (to Supabase, for the website)
// before alerting, and starts the retry loop for saves that fail. Call it
// before Start.
func (p *Processor) UseStore(saver SignalSaver) {
	p.store = saver
	p.pending = newSaveQueue(maxQueuedSaves)
}

// StartRetries retries failed saves every minute until ctx is cancelled.
func (p *Processor) StartRetries(ctx context.Context) {
	if p.store != nil {
		go p.pending.runRetries(ctx, p.store, time.Minute)
	}
}

// dispatchSignal saves the signal, then sends the Discord alert.
//
// The in-memory StateRegistry already stops repeats within this process; the
// saved row's unique key (signal, stock, day) also stops repeats across engine
// restarts, so a duplicate save means "already alerted today" and is skipped.
// If the save fails, the alert still goes out and the save is retried later:
// a Supabase outage must never cost an alert.
// It runs in its own goroutine to avoid blocking the worker pool.
func (p *Processor) dispatchSignal(signal domain.OrderSignal) {
	log.Printf("[processor] Signal fired: %s", signal.String())

	if p.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		inserted, err := p.store.InsertSignal(ctx, signal)
		cancel()
		switch {
		case err != nil:
			log.Printf("[processor] WARNING: could not save signal (will retry): %v", err)
			if !p.pending.add(signal) {
				log.Printf("[processor] WARNING: retry queue full — dropping save for %s %s", signal.TradingSymbol, signal.StrategyName)
			}
		case !inserted:
			log.Printf("[processor] %s %s already saved today — skipping duplicate alert", signal.TradingSymbol, signal.StrategyName)
			return
		}
	}

	if err := p.discord.SendSignal(signal); err != nil {
		log.Printf("[processor] ERROR sending Discord alert for token %d (strategy: %s): %v",
			signal.InstrumentToken, signal.StrategyName, err)
		return
	}
	log.Printf("[processor] Discord alert delivered for token %d", signal.InstrumentToken)
}
