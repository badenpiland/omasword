package esv

import (
	"strings"
	"testing"
	"time"
)

func TestLimiterMinuteHourAndDay(t *testing.T) {
	l, err := OpenLimiter("")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for i := 0; i < perMinute; i++ {
		if err := l.Acquire(start.Add(time.Duration(i) * time.Millisecond)); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	err = l.Acquire(start.Add(time.Second))
	rate, ok := err.(*RateError)
	if !ok {
		t.Fatalf("61st request err = %v, want RateError", err)
	}
	if strings.Contains(rate.Error(), "secret") {
		t.Fatal("rate error should not carry a key")
	}
	// The oldest hit is about a second old, so the minute window opens again
	// just under a minute later.
	if err := l.Acquire(start.Add(time.Minute + time.Second)); err != nil {
		t.Fatalf("after a minute: %v", err)
	}

	l.file.Hits = make([]int64, perDay)
	for i := range l.file.Hits {
		l.file.Hits[i] = start.UnixNano()
	}
	if err := l.Acquire(start.Add(2 * time.Minute)); err == nil {
		t.Fatal("daily cap should refuse the next request")
	}
	if err := l.Acquire(start.Add(25 * time.Hour)); err != nil {
		t.Fatalf("a day later: %v", err)
	}
}

func TestPenalizeBlocksUntilRetryAfter(t *testing.T) {
	l, err := OpenLimiter("")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	l.Penalize(now, 30*time.Second)
	if err := l.Acquire(now.Add(10 * time.Second)); err == nil {
		t.Fatal("a 429 penalty should block the next request")
	}
	if err := l.Acquire(now.Add(31 * time.Second)); err != nil {
		t.Fatalf("after the penalty: %v", err)
	}
}
