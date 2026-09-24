package service

import "testing"

func TestAlertGateOncePerOutage(t *testing.T) {
	var g alertGate
	if !g.shouldAlert(0) {
		t.Error("first error on a connection should alert")
	}
	for i := 0; i < 50; i++ {
		if g.shouldAlert(0) {
			t.Fatal("repeated errors during the same outage must not alert again")
		}
	}
	if !g.shouldAlert(1) {
		t.Error("each connection alerts independently")
	}
	g.reset(0) // connection 0 reconnected
	if !g.shouldAlert(0) {
		t.Error("after a successful reconnect, the next outage alerts again")
	}
}
