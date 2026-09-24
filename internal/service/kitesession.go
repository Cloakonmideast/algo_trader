package service

import (
	"context"
	"log"
	"sync"
	"time"

	"algotrader/internal/domain"
	"algotrader/internal/transport/supabase"
)

// TokenSource returns the Kite session the website's "Log in to Kite" button
// saved. Implemented by *supabase.Client.
type TokenSource interface {
	LatestToken(ctx context.Context) (supabase.KiteSession, error)
}

// reminderEvery spaces out "Kite login needed" reminders.
const reminderEvery = 30 * time.Minute

type sessionState struct {
	current      string // token in use; "" when none
	lastReminder time.Time
}

type sessionAction struct {
	useToken string // switch to this token
	stopFeed bool   // the token in use has expired
	remind   bool   // send a "Kite login needed" reminder
}

// decideSession is the watcher's whole policy, kept pure so it can be tested:
// use a new current token; drop an expired one; remind during market hours.
func decideSession(st sessionState, latest supabase.KiteSession, now time.Time) sessionAction {
	if latest.Found && latest.AccessToken != "" && domain.TokenCurrent(latest.CreatedAt, now) {
		if latest.AccessToken != st.current {
			return sessionAction{useToken: latest.AccessToken}
		}
		return sessionAction{}
	}
	return sessionAction{
		stopFeed: st.current != "",
		remind:   inReminderWindow(now) && now.Sub(st.lastReminder) >= reminderEvery,
	}
}

// inReminderWindow is 08:00–15:30 IST, Monday to Friday.
func inReminderWindow(now time.Time) bool {
	t := now.In(domain.IST)
	if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	mins := t.Hour()*60 + t.Minute()
	return mins >= 8*60 && mins < 15*60+30
}

// SessionWatcher keeps the engine on the Kite token saved by the website.
type SessionWatcher struct {
	src      TokenSource
	interval time.Duration
	now      func() time.Time

	// Callbacks, run on the watcher's goroutine.
	OnToken   func(token string) // a new token is current: apply it and (re)connect
	OnExpired func()             // the token in use expired: stop the live feed
	OnRemind  func()             // no current token during market hours

	mu    sync.Mutex
	state sessionState
}

func NewSessionWatcher(src TokenSource, interval time.Duration) *SessionWatcher {
	return &SessionWatcher{
		src: src, interval: interval, now: time.Now,
		OnToken: func(string) {}, OnExpired: func() {}, OnRemind: func() {},
	}
}

// Valid reports whether the engine currently holds a current token.
func (w *SessionWatcher) Valid() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state.current != ""
}

// WaitForToken blocks until a current token exists and returns it, reminding
// during market hours. It is used at startup, before anything talks to Kite.
func (w *SessionWatcher) WaitForToken(ctx context.Context) (string, error) {
	logged := false
	for {
		if tok, ok := w.poll(ctx, false); ok {
			return tok, nil
		}
		if !logged {
			log.Println("[session] Waiting for today's Kite login (use \"Log in to Kite\" on the website)")
			logged = true
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(w.interval):
		}
	}
}

// Run applies token changes every interval until ctx is cancelled.
func (w *SessionWatcher) Run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.step(ctx)
		}
	}
}

func (w *SessionWatcher) step(ctx context.Context) { w.poll(ctx, true) }

// poll fetches the latest session and acts on it. When apply is false (startup)
// a new token is adopted without calling OnToken. It returns the token in use.
func (w *SessionWatcher) poll(ctx context.Context, apply bool) (string, bool) {
	latest, err := w.src.LatestToken(ctx)
	if err != nil {
		log.Printf("[session] WARNING: could not read the Kite token from Supabase: %v", err)
		return "", false
	}
	now := w.now()

	w.mu.Lock()
	act := decideSession(w.state, latest, now)
	if act.useToken != "" {
		w.state.current = act.useToken
	}
	if act.stopFeed {
		w.state.current = ""
	}
	if act.remind {
		w.state.lastReminder = now
	}
	current := w.state.current
	w.mu.Unlock()

	switch {
	case act.useToken != "" && apply:
		log.Println("[session] New Kite token from the website — applying")
		w.OnToken(act.useToken)
	case act.stopFeed:
		log.Println("[session] Kite token expired — stopping the live feed until the next login")
		w.OnExpired()
	}
	if act.remind {
		w.OnRemind()
	}
	return current, current != ""
}
