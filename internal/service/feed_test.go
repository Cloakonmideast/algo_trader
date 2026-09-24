package service

import "testing"

type fakeTicker struct {
	token            string
	started, stopped int
	connected        int
}

func (f *fakeTicker) Start()         { f.started++ }
func (f *fakeTicker) Stop()          { f.stopped++ }
func (f *fakeTicker) Connected() int { return f.connected }

func TestLiveFeedReplacesAndStops(t *testing.T) {
	var made []*fakeTicker
	feed := NewLiveFeed(func(token string) Ticker {
		ft := &fakeTicker{token: token}
		made = append(made, ft)
		return ft
	})

	if feed.Connected() {
		t.Error("a feed that was never started is not connected")
	}
	feed.Start("A")
	made[0].connected = 3
	if !feed.Connected() {
		t.Error("expected connected after the ticker reports connections")
	}

	feed.Start("B") // new token: old ticker stopped, new one started
	if made[0].stopped != 1 || made[1].started != 1 || made[1].token != "B" {
		t.Errorf("replace: old stopped=%d, new started=%d token=%q", made[0].stopped, made[1].started, made[1].token)
	}

	feed.Stop()
	feed.Stop() // idempotent
	if made[1].stopped != 1 || feed.Connected() {
		t.Errorf("stop: stopped=%d connected=%v", made[1].stopped, feed.Connected())
	}
}
