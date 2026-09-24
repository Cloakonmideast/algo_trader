package supabase

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"algotrader/internal/domain"
)

// fakeServer records the last request and answers with the queued responses in order.
type fakeServer struct {
	t         *testing.T
	responses []fakeResponse
	calls     atomic.Int32
	lastReq   *http.Request
	lastBody  string
}

type fakeResponse struct {
	status int
	body   string
}

func (f *fakeServer) handler(w http.ResponseWriter, r *http.Request) {
	i := int(f.calls.Add(1)) - 1
	b, _ := io.ReadAll(r.Body)
	f.lastReq, f.lastBody = r, string(b)
	resp := f.responses[len(f.responses)-1]
	if i < len(f.responses) {
		resp = f.responses[i]
	}
	w.WriteHeader(resp.status)
	_, _ = w.Write([]byte(resp.body))
}

func newTestClient(t *testing.T, responses ...fakeResponse) (*Client, *fakeServer) {
	f := &fakeServer{t: t, responses: responses}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "sb_secret_test")
	c.backoff = func(int) time.Duration { return 0 }
	return c, f
}

func rsiSignal() domain.OrderSignal {
	return domain.OrderSignal{
		InstrumentToken: 408065,
		TradingSymbol:   "INFY",
		StrategyName:    "RSI (Daily Strong Bullish)",
		SignalTime:      time.Date(2026, 9, 24, 20, 0, 0, 0, time.UTC), // 01:30 IST on the 25th
		Strategy:        "RSI",
		Price:           1523.4,
		Direction:       domain.Long,
		RSI:             &domain.RSIReadings{Daily: 62.13, Weekly: 64.3, WeeklyOK: true},
	}
}

func TestInsertSignalNewRow(t *testing.T) {
	c, f := newTestClient(t, fakeResponse{201, `[{"id":1}]`})
	inserted, err := c.InsertSignal(context.Background(), rsiSignal())
	if err != nil || !inserted {
		t.Fatalf("inserted=%v err=%v, want true, nil", inserted, err)
	}

	r := f.lastReq
	if r.Method != http.MethodPost || r.URL.Path != "/rest/v1/signals" || r.URL.Query().Get("on_conflict") != "signal,token,trade_date" {
		t.Errorf("request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
	}
	if r.Header.Get("apikey") != "sb_secret_test" || r.Header.Get("Authorization") != "" {
		t.Errorf("new-style secret key: apikey=%q authorization=%q, want apikey only", r.Header.Get("apikey"), r.Header.Get("Authorization"))
	}
	if p := r.Header.Get("Prefer"); !strings.Contains(p, "resolution=ignore-duplicates") || !strings.Contains(p, "return=representation") {
		t.Errorf("Prefer = %q", p)
	}

	var row map[string]any
	if err := json.Unmarshal([]byte(f.lastBody), &row); err != nil {
		t.Fatalf("body is not a JSON object: %v (%s)", err, f.lastBody)
	}
	want := map[string]any{
		"trade_date": "2026-09-25", "token": float64(408065), "symbol": "INFY", "strategy": "RSI",
		"signal": "RSI (Daily Strong Bullish)", "direction": "long", "price": 1523.4,
		"rsi_daily": 62.13, "rsi_weekly": 64.3, "rsi_monthly": nil,
	}
	for k, v := range want {
		if row[k] != v {
			t.Errorf("row[%q] = %v, want %v", k, row[k], v)
		}
	}
	if row["fired_at"] != "2026-09-24T20:00:00Z" {
		t.Errorf("fired_at = %v", row["fired_at"])
	}
}

func TestInsertSignalDuplicate(t *testing.T) {
	c, _ := newTestClient(t, fakeResponse{201, `[]`})
	inserted, err := c.InsertSignal(context.Background(), rsiSignal())
	if err != nil || inserted {
		t.Fatalf("inserted=%v err=%v, want false, nil (already saved today)", inserted, err)
	}
}

func TestInsertSignalNonRSIHasNullReadings(t *testing.T) {
	c, f := newTestClient(t, fakeResponse{201, `[{"id":1}]`})
	s := rsiSignal()
	s.RSI = nil
	if _, err := c.InsertSignal(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{`"rsi_daily":null`, `"rsi_weekly":null`, `"rsi_monthly":null`} {
		if !strings.Contains(f.lastBody, k) {
			t.Errorf("body missing %s: %s", k, f.lastBody)
		}
	}
}

func TestRetriesServerErrorsButNotClientErrors(t *testing.T) {
	c, f := newTestClient(t, fakeResponse{503, "busy"}, fakeResponse{201, `[{"id":1}]`})
	if inserted, err := c.InsertSignal(context.Background(), rsiSignal()); err != nil || !inserted {
		t.Fatalf("after one 503: inserted=%v err=%v", inserted, err)
	}
	if f.calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", f.calls.Load())
	}

	c, f = newTestClient(t, fakeResponse{400, `{"message":"bad column"}`})
	if _, err := c.InsertSignal(context.Background(), rsiSignal()); err == nil || !strings.Contains(err.Error(), "bad column") {
		t.Fatalf("err = %v, want the server's message", err)
	}
	if f.calls.Load() != 1 {
		t.Errorf("a 400 must not be retried; calls = %d", f.calls.Load())
	}

	c, f = newTestClient(t, fakeResponse{500, "down"})
	if _, err := c.InsertSignal(context.Background(), rsiSignal()); err == nil {
		t.Fatal("persistent 500 should return an error")
	}
	if f.calls.Load() != maxAttempts {
		t.Errorf("calls = %d, want %d", f.calls.Load(), maxAttempts)
	}
}

func TestLatestToken(t *testing.T) {
	c, f := newTestClient(t, fakeResponse{200, `[{"access_token":"tok123","user_id":"AB1234","created_at":"2026-09-25T02:30:00.123+00:00"}]`})
	s, err := c.LatestToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !s.Found || s.AccessToken != "tok123" || s.UserID != "AB1234" || !s.CreatedAt.Equal(time.Date(2026, 9, 25, 2, 30, 0, 123e6, time.UTC)) {
		t.Errorf("session = %+v", s)
	}
	if f.lastReq.Method != http.MethodGet || f.lastReq.URL.Query().Get("id") != "eq.1" {
		t.Errorf("request = %s %s", f.lastReq.Method, f.lastReq.URL.RawQuery)
	}

	c, _ = newTestClient(t, fakeResponse{200, `[]`})
	if s, err := c.LatestToken(context.Background()); err != nil || s.Found {
		t.Errorf("empty table: session=%+v err=%v, want not found", s, err)
	}
}

func TestHeartbeatUpserts(t *testing.T) {
	c, f := newTestClient(t, fakeResponse{201, ``})
	err := c.Heartbeat(context.Background(), EngineStatus{
		LastSeen: time.Date(2026, 9, 25, 4, 0, 0, 0, time.UTC), Instruments: 500, FeedConnected: true, TokenValid: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastReq.URL.Path != "/rest/v1/engine_status" || !strings.Contains(f.lastReq.Header.Get("Prefer"), "resolution=merge-duplicates") {
		t.Errorf("request = %s, Prefer = %q", f.lastReq.URL.Path, f.lastReq.Header.Get("Prefer"))
	}
	for _, k := range []string{`"id":1`, `"instruments":500`, `"feed_connected":true`, `"token_valid":true`, `"last_seen":"2026-09-25T04:00:00Z"`} {
		if !strings.Contains(f.lastBody, k) {
			t.Errorf("body missing %s: %s", k, f.lastBody)
		}
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_SERVICE_KEY", "")
	if FromEnv() != nil {
		t.Error("FromEnv should return nil when unconfigured")
	}
	t.Setenv("SUPABASE_URL", "https://x.supabase.co/")
	t.Setenv("SUPABASE_SERVICE_KEY", "k")
	c := FromEnv()
	if c == nil || c.baseURL != "https://x.supabase.co" {
		t.Errorf("FromEnv = %+v", c)
	}
}

func TestLegacyJWTKeyAlsoSentAsBearer(t *testing.T) {
	f := &fakeServer{t: t, responses: []fakeResponse{{200, "[]"}}}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(srv.Close)
	c := New(srv.URL, "eyJhbGciOiJIUzI1NiJ9.legacy.sig")
	if _, err := c.LatestToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := f.lastReq.Header.Get("Authorization"); got != "Bearer eyJhbGciOiJIUzI1NiJ9.legacy.sig" {
		t.Errorf("legacy service_role key: Authorization = %q", got)
	}
}
