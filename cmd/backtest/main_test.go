package main

import (
	"testing"
	"time"

	"algotrader/internal/domain"
)

// -from and -to name IST trading days, and sessions are timestamped at IST
// midnight, so the flag must parse to IST midnight: read as UTC, -from would
// skip its own day.
func TestParseDateIsISTMidnight(t *testing.T) {
	got, err := parseDate("2026-09-18")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.September, 18, 0, 0, 0, 0, domain.IST); !got.Equal(want) {
		t.Errorf("parseDate = %v, want %v", got, want)
	}
	if got, err := parseDate(""); err != nil || !got.IsZero() {
		t.Errorf("parseDate(\"\") = %v, %v; want zero time", got, err)
	}
}
