// Package supabase is the engine's small REST client for the Supabase project
// that backs the website: it saves signals, reads the daily Kite token saved by
// the site's "Log in to Kite" button, and writes the engine heartbeat.
//
// It talks to PostgREST directly with the project's secret (service_role) key,
// so it needs no Supabase SDK. The schema is supabase/migrations/001_init.sql.
package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"algotrader/internal/domain"
)

// maxAttempts bounds retries of timeouts, network errors, 429s and 5xx responses.
const maxAttempts = 3

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	backoff func(attempt int) time.Duration
}

func New(baseURL, key string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    &http.Client{Timeout: 10 * time.Second},
		backoff: func(attempt int) time.Duration { return time.Duration(attempt) * time.Second },
	}
}

// FromEnv builds a client from SUPABASE_URL and SUPABASE_SERVICE_KEY, or
// returns nil when either is unset.
func FromEnv() *Client {
	url := strings.TrimSpace(os.Getenv("SUPABASE_URL"))
	key := strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_KEY"))
	if url == "" || key == "" {
		return nil
	}
	return New(url, key)
}

// signalRow is one row of the signals table.
type signalRow struct {
	FiredAt    time.Time `json:"fired_at"`
	TradeDate  string    `json:"trade_date"`
	Token      uint32    `json:"token"`
	Symbol     string    `json:"symbol"`
	Strategy   string    `json:"strategy"`
	Signal     string    `json:"signal"`
	Direction  string    `json:"direction"`
	Price      float64   `json:"price"`
	RSIDaily   *float64  `json:"rsi_daily"`
	RSIWeekly  *float64  `json:"rsi_weekly"`
	RSIMonthly *float64  `json:"rsi_monthly"`
}

func toRow(s domain.OrderSignal) signalRow {
	row := signalRow{
		FiredAt:   s.SignalTime.UTC(),
		TradeDate: domain.TradeDate(s.SignalTime),
		Token:     s.InstrumentToken,
		Symbol:    s.TradingSymbol,
		Strategy:  s.Strategy,
		Signal:    s.StrategyName,
		Direction: s.Direction.String(),
		Price:     s.Price,
	}
	if r := s.RSI; r != nil {
		row.RSIDaily = &r.Daily
		if r.WeeklyOK {
			row.RSIWeekly = &r.Weekly
		}
		if r.MonthlyOK {
			row.RSIMonthly = &r.Monthly
		}
	}
	return row
}

// InsertSignal saves a signal. It returns inserted=false, with no error, when
// the same signal already exists for that stock and trading day (the table's
// unique key), which is how an engine restart avoids re-alerting.
func (c *Client) InsertSignal(ctx context.Context, s domain.OrderSignal) (inserted bool, err error) {
	body, err := json.Marshal(toRow(s))
	if err != nil {
		return false, err
	}
	resp, err := c.do(ctx, http.MethodPost, "/rest/v1/signals?on_conflict=signal,token,trade_date",
		"resolution=ignore-duplicates,return=representation", body)
	if err != nil {
		return false, err
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(resp, &rows); err != nil {
		return false, fmt.Errorf("supabase: unexpected insert response %q: %w", truncate(resp), err)
	}
	return len(rows) > 0, nil
}

// KiteSession is the row the website's Kite callback saves.
type KiteSession struct {
	Found       bool
	AccessToken string
	UserID      string
	CreatedAt   time.Time
}

func (c *Client) LatestToken(ctx context.Context) (KiteSession, error) {
	resp, err := c.do(ctx, http.MethodGet, "/rest/v1/kite_session?id=eq.1&select=access_token,user_id,created_at", "", nil)
	if err != nil {
		return KiteSession{}, err
	}
	var rows []struct {
		AccessToken string    `json:"access_token"`
		UserID      string    `json:"user_id"`
		CreatedAt   time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(resp, &rows); err != nil {
		return KiteSession{}, fmt.Errorf("supabase: unexpected kite_session response %q: %w", truncate(resp), err)
	}
	if len(rows) == 0 {
		return KiteSession{}, nil
	}
	r := rows[0]
	return KiteSession{Found: true, AccessToken: r.AccessToken, UserID: r.UserID, CreatedAt: r.CreatedAt}, nil
}

// EngineStatus is the heartbeat shown in the website header.
type EngineStatus struct {
	LastSeen      time.Time
	Instruments   int
	FeedConnected bool
	TokenValid    bool
}

func (c *Client) Heartbeat(ctx context.Context, st EngineStatus) error {
	body, err := json.Marshal(map[string]any{
		"id":             1,
		"last_seen":      st.LastSeen.UTC(),
		"instruments":    st.Instruments,
		"feed_connected": st.FeedConnected,
		"token_valid":    st.TokenValid,
	})
	if err != nil {
		return err
	}
	_, err = c.do(ctx, http.MethodPost, "/rest/v1/engine_status?on_conflict=id", "resolution=merge-duplicates,return=minimal", body)
	return err
}

// do sends one PostgREST request, retrying transient failures.
func (c *Client) do(ctx context.Context, method, path, prefer string, body []byte) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.backoff(attempt - 1)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		// The apikey header alone works for both key styles. Legacy service_role
		// keys are JWTs and may also go in Authorization; new sb_secret_… keys
		// are not JWTs, so they must not.
		req.Header.Set("apikey", c.key)
		if strings.HasPrefix(c.key, "eyJ") {
			req.Header.Set("Authorization", "Bearer "+c.key)
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if prefer != "" {
			req.Header.Set("Prefer", prefer)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("supabase: %s %s: %w", method, path, err)
			continue
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("supabase: reading response: %w", readErr)
			continue
		}

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return data, nil
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("supabase: %s %s: HTTP %d: %s", method, path, resp.StatusCode, truncate(data))
			continue
		default:
			return nil, fmt.Errorf("supabase: %s %s: HTTP %d: %s", method, path, resp.StatusCode, truncate(data))
		}
	}
	return nil, lastErr
}

func truncate(b []byte) string {
	const max = 300
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}
