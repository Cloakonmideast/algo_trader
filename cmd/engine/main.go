package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"algotrader/internal/config"
	"algotrader/internal/domain"
	"algotrader/internal/logger"
	"algotrader/internal/repository"
	"algotrader/internal/scheduler"
	"algotrader/internal/service"
	"algotrader/internal/strategy"
	"algotrader/internal/transport/discord"
	"algotrader/internal/transport/supabase"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"
)

func main() {
	// ── Logging Setup ─────────────────────────────────────────────────────────
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(os.Stdout)
	printBanner()

	// Initialise structured JSON logging. After this call every log.Printf line
	// is written to stdout (plain text) AND to logs/algotrader.log (JSON lines).
	if err := logger.Init("logs/algotrader.log"); err != nil {
		log.Printf("[main] WARNING: could not initialise JSON log file: %v", err)
	}

	// Cancelled on SIGINT/SIGTERM, including while waiting for the day's Kite login.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── 1. Load Configuration (fail-fast on missing env vars) ─────────────────
	log.Println("[main] Loading configuration from environment variables")
	cfg := config.Load()

	if cfg.ScanNifty500 {
		log.Println("[main] Mode: NIFTY 500 SCAN — constituents will be fetched from NSE India CSV")
	} else if cfg.ScanFullNSE {
		log.Println("[main] Mode: FULL NSE SCAN — instrument tokens will be auto-discovered")
	} else {
		log.Printf("[main] Mode: MANUAL — tracking %d instrument tokens", len(cfg.InstrumentTokens))
	}

	// ── 2. Initialize SQLite Database ─────────────────────────────────────────
	log.Printf("[main] Opening SQLite database: %s", cfg.DatabasePath)
	db, err := repository.NewDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("[main] FATAL: failed to initialize database: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("[main] WARNING: error closing database during shutdown: %v", closeErr)
		}
	}()

	// ── 3. Initialize In-Process RAM Cache ────────────────────────────────────
	log.Println("[main] Initializing MarketContext RAM cache")
	mktCtx := domain.NewMarketContext()

	// ── 4. Discord (lifecycle alerts, signal alerts, login reminders) ─────────
	log.Println("[main] Initializing Discord webhook transport client")
	discordClient := discord.NewClient(cfg.DiscordWebhookURL)

	// ── 5. Kite access token ──────────────────────────────────────────────────
	// With Supabase configured, the token comes from the website's "Log in to
	// Kite" button and the engine waits for it; otherwise KITE_ACCESS_TOKEN.
	var (
		supa        *supabase.Client
		watcher     *service.SessionWatcher
		feed        atomic.Pointer[service.LiveFeed]
		instruments atomic.Int64
		accessToken = cfg.KiteAccessToken
	)
	if cfg.UseSupabase() {
		log.Println("[main] Supabase configured — signals go to the website; the Kite token comes from its login button")
		supa = supabase.New(cfg.SupabaseURL, cfg.SupabaseServiceKey)
		watcher = service.NewSessionWatcher(supa, time.Minute)
		watcher.OnRemind = func() {
			_ = discordClient.SendAlert("Kite login needed", loginHint(cfg.SiteURL), true)
		}
		watcher.OnSourceError = func(err error) {
			_ = discordClient.SendAlert("Can't read Supabase",
				fmt.Sprintf("The engine can't reach the website's database, so it can't get the Kite token or save signals. Check SUPABASE_URL and SUPABASE_SERVICE_KEY in the server's .env. Error: %v", err), true)
		}
		// The heartbeat starts now so the website shows the engine alive while it waits.
		go runHeartbeat(ctx, supa, watcher, &feed, &instruments)

		accessToken, err = watcher.WaitForToken(ctx)
		if err != nil {
			log.Println("[main] Shutdown requested while waiting for the Kite login")
			return
		}
		log.Println("[main] Kite token received from the website")
	}

	// ── 6. Initialize Zerodha Kite Connect REST Client ────────────────────────
	log.Println("[main] Initializing Zerodha Kite Connect API client")
	kc := kiteconnect.New(cfg.KiteAPIKey)
	kc.SetAccessToken(accessToken)

	// ── 7. Resolve Instrument Token List ──────────────────────────────────────
	var tokens []uint32
	if cfg.ScanNifty500 {
		log.Println("[main] Discovering Nifty 500 instruments...")
		tokens, err = service.FetchNifty500Tokens(kc)
		if err != nil {
			log.Fatalf("[main] FATAL: Nifty 500 instrument discovery failed: %v", err)
		}
		log.Printf("[main] Discovered %d Nifty 500 instruments", len(tokens))
	} else if cfg.ScanFullNSE {
		log.Println("[main] Discovering all NSE EQ instruments via Zerodha API...")
		tokens, err = service.FetchNSEEquityTokens(kc)
		if err != nil {
			log.Fatalf("[main] FATAL: instrument discovery failed: %v", err)
		}
		log.Printf("[main] Discovered %d NSE EQ instruments", len(tokens))
	} else {
		tokens = cfg.InstrumentTokens
	}
	instruments.Store(int64(len(tokens)))

	// ── 7b. Build Token → Symbol Lookup Map ───────────────────────────────────
	// The Zerodha WebSocket binary packet does not carry the trading symbol;
	// we resolve it here from the instrument master so alerts show real names.
	log.Println("[main] Building token-to-symbol lookup map from Zerodha instrument master")
	symbolMap := make(map[uint32]string, len(tokens))
	if instrumentList, instErr := kc.GetInstrumentsByExchange("NSE"); instErr != nil {
		log.Printf("[main] WARNING: could not fetch instrument master for symbol resolution: %v", instErr)
	} else {
		for _, inst := range instrumentList {
			symbolMap[uint32(inst.InstrumentToken)] = inst.Tradingsymbol
		}
		log.Printf("[main] Symbol map built: %d entries", len(symbolMap))
	}

	// ── 8. Initialize Domain StateRegistry ────────────────────────────────────
	log.Println("[main] Initializing StateRegistry for concurrent signal deduplication")
	registry := domain.NewStateRegistry()

	// ── 9. Build Tick Processor ───────────────────────────────────────────────
	log.Printf("[main] Building tick processor: %d workers, channel capacity %d",
		cfg.WorkerPoolSize, cfg.TickChannelCapacity)
	proc := service.NewProcessor(
		cfg.TickChannelCapacity,
		cfg.WorkerPoolSize,
		registry,
		mktCtx,
		discordClient, // webhook: sends stock+strategy name alerts
		symbolMap,
	)
	if supa != nil {
		// Save every signal for the website before alerting; a duplicate save
		// means it was already alerted today (e.g. before a restart).
		proc.UseStore(supa)
	}

	// Register strategies — add more implementations here as needed.
	proc.RegisterStrategy(strategy.NewIchimokuKinkoHyo())

	// RSI emits the four GFS setups (bullish and bearish) and the daily and
	// weekly range signals — see strategy.RSI for the rules. All of them alert.
	//
	// The allowlist is kept as a standing guarantee that a signal added to the
	// strategy later cannot silently start alerting.
	//
	// The StateRegistry caps delivery at one alert per signal name per stock per
	// day, so a weekly and a daily signal on the same stock both get through.
	proc.RegisterStrategy(strategy.OnlySignals(
		strategy.NewRSI(14),
		strategy.RSISignalNames()...,
	))

	// ── 10. Initialize Bootloader ─────────────────────────────────────────────
	log.Println("[main] Initializing pre-market bootloader")
	bootloader := service.NewBootloader(db, mktCtx, kc)

	// ── 11. Pre-market Boot Sequence ──────────────────────────────────────────
	// Populates the RAM cache from SQLite (fast path) or Zerodha API (first-run path).
	log.Printf("[main] Executing pre-market boot sequence for %d tokens", len(tokens))
	bootloader.Boot(tokens)
	log.Printf("[main] Boot complete: RAM cache has data for %d tokens", mktCtx.TokenCount())

	// ── 11b. Purge Delisted Instruments from Database ─────────────────────────
	// Remove candle rows for any token no longer present in the live NSE EQ list.
	if cfg.ScanFullNSE || cfg.ScanNifty500 {
		log.Println("[main] Running delisted instrument cleanup...")
		deleted, err := db.CleanupDelisted(tokens)
		if err != nil {
			log.Printf("[main] WARNING: CleanupDelisted failed: %v", err)
		} else {
			log.Printf("[main] Cleanup complete: removed %d candle rows for delisted instruments", deleted)
		}
	}

	// ── 12. Initialize & Start Market Scheduler ───────────────────────────────
	log.Println("[main] Initializing market lifecycle scheduler")
	sched := scheduler.NewMarketScheduler(bootloader, tokens)
	sched.Start()

	// ── 13. Start Tick Processor Worker Pool ──────────────────────────────────
	log.Println("[main] Starting tick processor worker pool")
	proc.Start()
	proc.StartRetries(ctx)

	// ── 14. Live feed: 3 WebSocket connections ────────────────────────────────
	// MultiTicker distributes all tokens across 3 independent Zerodha WebSocket
	// connections (Zerodha's per-API-key limit). A WebSocket authenticates once,
	// so LiveFeed rebuilds it whenever the Kite token changes.
	log.Printf("[main] Initializing live feed: 3 WebSocket connections for %d tokens", len(tokens))
	liveFeed := service.NewLiveFeed(func(token string) service.Ticker {
		return service.NewMultiTicker(cfg.KiteAPIKey, token, tokens, proc.Enqueue, discordClient)
	})
	feed.Store(liveFeed)
	liveFeed.Start(accessToken)

	// ── 15. Follow the website's Kite login ───────────────────────────────────
	if watcher != nil {
		watcher.OnToken = func(tok string) {
			kc.SetAccessToken(tok)
			liveFeed.Start(tok)
			_ = discordClient.SendAlert("Kite token updated",
				"New login from the website — live feed reconnected. Refreshing candle data.", false)
			bootloader.Boot(tokens)
		}
		watcher.OnExpired = func() {
			liveFeed.Stop()
			_ = discordClient.SendAlert("Kite session expired",
				"Live feed stopped until the next login. "+loginHint(cfg.SiteURL), true)
		}
		go watcher.Run(ctx)
	}

	// Send a single consolidated Discord alert after all WS connections initialise.
	_ = discordClient.SendAlert(
		"AlgoTrader Online",
		fmt.Sprintf("Engine started. Monitoring %d instruments across 3 WebSocket connections.", len(tokens)),
		false,
	)

	// ── 16. Block until SIGINT/SIGTERM — Graceful Shutdown ────────────────────
	<-ctx.Done()
	log.Println("[main] Shutdown signal received — initiating graceful shutdown")
	_ = discordClient.SendAlert("AlgoTrader Shutting Down", "Shutdown signal received. Graceful shutdown initiated.", true)

	// Shutdown sequence (reverse of initialisation order):

	// 1. Stop the live feed to prevent new ticks from arriving.
	log.Println("[main] Step 1/4: Closing live feed (3 WebSocket connections)")
	liveFeed.Stop()

	// 2. Stop the cron scheduler (waits for any in-progress job to finish).
	log.Println("[main] Step 2/4: Stopping market lifecycle scheduler")
	sched.Stop()

	// 3. Stop the processor (closes channel, drains remaining ticks, waits for workers).
	log.Println("[main] Step 3/4: Stopping tick processor worker pool")
	proc.Stop()

	// 4. Database is closed by the deferred db.Close() at function return.
	log.Println("[main] Step 4/4: Flushing database via deferred close handler")

	log.Println("[main] AlgoTrader engine shut down cleanly. Goodbye.")

	// Flush and close the JSON log file last — after all goroutines have stopped writing.
	if err := logger.Close(); err != nil {
		log.Printf("[main] WARNING: error closing log file: %v", err)
	}
}

// runHeartbeat tells the website the engine is alive, once a minute, until ctx
// is cancelled. It also keeps the free Supabase project from pausing.
func runHeartbeat(ctx context.Context, supa *supabase.Client, watcher *service.SessionWatcher,
	feed *atomic.Pointer[service.LiveFeed], instruments *atomic.Int64) {
	beat := func() {
		connected := false
		if f := feed.Load(); f != nil {
			connected = f.Connected()
		}
		hctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		err := supa.Heartbeat(hctx, supabase.EngineStatus{
			LastSeen:      time.Now(),
			Instruments:   int(instruments.Load()),
			FeedConnected: connected,
			TokenValid:    watcher.Valid(),
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("[main] WARNING: heartbeat failed: %v", err)
		}
	}

	beat()
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			beat()
		}
	}
}

func loginHint(siteURL string) string {
	if siteURL == "" {
		return "Open the website and click \"Log in to Kite\"."
	}
	return fmt.Sprintf("Log in: %s (click \"Log in to Kite\").", siteURL)
}

// printBanner outputs the application startup header.
func printBanner() {
	fmt.Print(`
 ┌─────────────────────────────────────────────────┐
 │          A L G O T R A D E R   ENGINE           │
 │   Zerodha Kite Connect  •  SQLite  •  Discord   │
 │   Low-Latency Concurrent Algorithmic Trading    │
 └─────────────────────────────────────────────────┘

`)
}
