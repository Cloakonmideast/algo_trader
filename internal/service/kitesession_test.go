package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"algotrader/internal/transport/supabase"
)

func ist(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		t.Fatal(err)
	}
	loc := time.FixedZone("IST", 5*3600+30*60)
	return time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), 0, 0, loc)
}

func session(token string, created time.Time) supabase.KiteSession {
	return supabase.KiteSession{Found: true, AccessToken: token, UserID: "AB1234", CreatedAt: created}
}

func TestDecideSession(t *testing.T) {
	thu0930 := ist(t, "2026-09-24 09:30") // Thursday
	today0800 := ist(t, "2026-09-24 08:00")
	yesterday := ist(t, "2026-09-23 09:00")

	cases := []struct {
		name   string
		state  sessionState
		latest supabase.KiteSession
		now    time.Time
		want   sessionAction
	}{
		{"new token today → use it", sessionState{}, session("A", today0800), thu0930, sessionAction{useToken: "A"}},
		{"same token → nothing", sessionState{current: "A"}, session("A", today0800), thu0930, sessionAction{}},
		{"re-login gives a new token → switch", sessionState{current: "A"}, session("B", thu0930), thu0930, sessionAction{useToken: "B"}},
		{"token expired while in use → stop feed and remind", sessionState{current: "A"}, session("A", yesterday), thu0930, sessionAction{stopFeed: true, remind: true}},
		{"no token yet in market hours → remind", sessionState{}, supabase.KiteSession{}, thu0930, sessionAction{remind: true}},
		{"reminded 10 min ago → wait", sessionState{lastReminder: thu0930.Add(-10 * time.Minute)}, supabase.KiteSession{}, thu0930, sessionAction{}},
		{"reminded 30 min ago → remind again", sessionState{lastReminder: thu0930.Add(-30 * time.Minute)}, supabase.KiteSession{}, thu0930, sessionAction{remind: true}},
		{"before 08:00 → no reminder", sessionState{}, supabase.KiteSession{}, ist(t, "2026-09-24 07:59"), sessionAction{}},
		{"after 15:30 → no reminder", sessionState{}, supabase.KiteSession{}, ist(t, "2026-09-24 15:30"), sessionAction{}},
		{"Saturday → no reminder", sessionState{}, supabase.KiteSession{}, ist(t, "2026-09-26 10:00"), sessionAction{}},
	}
	for _, tc := range cases {
		if got := decideSession(tc.state, tc.latest, tc.now); got != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// scriptedSource returns the queued sessions in order, then repeats the last one.
type scriptedSource struct {
	mu    sync.Mutex
	queue []supabase.KiteSession
	errs  []error
}

func (s *scriptedSource) LatestToken(context.Context) (supabase.KiteSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	if len(s.errs) > 0 {
		err, s.errs = s.errs[0], s.errs[1:]
		if err != nil {
			return supabase.KiteSession{}, err
		}
	}
	v := s.queue[0]
	if len(s.queue) > 1 {
		s.queue = s.queue[1:]
	}
	return v, nil
}

func TestWaitForTokenPollsUntilLoggedIn(t *testing.T) {
	now := ist(t, "2026-09-24 09:30")
	src := &scriptedSource{
		errs:  []error{errors.New("network blip")},
		queue: []supabase.KiteSession{{}, session("A", now)},
	}
	reminders := 0
	w := NewSessionWatcher(src, time.Millisecond)
	w.now = func() time.Time { return now }
	w.OnRemind = func() { reminders++ }

	tok, err := w.WaitForToken(context.Background())
	if err != nil || tok != "A" {
		t.Fatalf("WaitForToken = %q, %v", tok, err)
	}
	if reminders != 1 {
		t.Errorf("reminders = %d, want 1 (then spaced 30 min apart)", reminders)
	}
	if !w.Valid() {
		t.Error("Valid() should be true once a token is in use")
	}
}

func TestWaitForTokenStopsOnCancel(t *testing.T) {
	w := NewSessionWatcher(&scriptedSource{queue: []supabase.KiteSession{{}}}, time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.WaitForToken(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestStepAppliesTokenChangesAndExpiry(t *testing.T) {
	now := ist(t, "2026-09-24 09:30")
	src := &scriptedSource{queue: []supabase.KiteSession{
		session("A", now), session("A", now), session("B", now), session("B", ist(t, "2026-09-23 09:00")),
	}}
	var used []string
	expired := 0
	w := NewSessionWatcher(src, time.Millisecond)
	w.now = func() time.Time { return now }
	w.OnToken = func(tok string) { used = append(used, tok) }
	w.OnExpired = func() { expired++ }
	w.OnRemind = func() {}

	for i := 0; i < 4; i++ {
		w.step(context.Background())
	}
	if len(used) != 2 || used[0] != "A" || used[1] != "B" {
		t.Errorf("tokens applied = %v, want [A B]", used)
	}
	if expired != 1 || w.Valid() {
		t.Errorf("expired=%d valid=%v, want 1 and false", expired, w.Valid())
	}
}
