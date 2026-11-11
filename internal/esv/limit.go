package esv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Crossway's published API limits: 60/minute, 1,000/hour, 5,000/day.
const (
	perMinute = 60
	perHour   = 1000
	perDay    = 5000
)

// RateError is a local or server rate limit. RetryAfter is how long to
// wait before another request. The message never includes an API key.
type RateError struct {
	RetryAfter time.Duration
}

func (e *RateError) Error() string {
	secs := int((e.RetryAfter + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	return fmt.Sprintf("ESV rate limit reached; try again in %ds", secs)
}

type limitFile struct {
	Hits         []int64 `json:"hits"`
	BlockedUntil int64   `json:"blocked_until,omitempty"`
}

// Limiter is a sliding-window counter for Crossway's three rate limits.
// An empty path keeps the counts in memory. Counts are shared across
// process runs when a path is set, so a script cannot reset the daily
// cap by exiting.
type Limiter struct {
	mu   sync.Mutex
	path string
	now  func() time.Time
	file limitFile
}

// OpenLimiter loads path, or an empty limiter when the file is missing.
func OpenLimiter(path string) (*Limiter, error) {
	l := &Limiter{path: path, now: time.Now}
	if path == "" {
		return l, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &l.file); err != nil {
		l.file = limitFile{}
	}
	return l, nil
}

// Acquire records one request, or returns a RateError when another
// request would pass a limit. It does not sleep.
func (l *Limiter) Acquire(now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.fileBlocked(now).IsZero() {
		return &RateError{RetryAfter: l.fileBlocked(now).Sub(now)}
	}
	l.prune(now)
	if wait, ok := l.room(now); !ok {
		return &RateError{RetryAfter: wait}
	}
	l.file.Hits = append(l.file.Hits, now.UnixNano())
	return l.save()
}

// Penalize blocks new requests until d has passed. Used when Crossway
// answers 429, so a retry loop does not keep calling the API.
func (l *Limiter) Penalize(now time.Time, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if d < time.Second {
		d = time.Minute
	}
	if d > 24*time.Hour {
		d = 24 * time.Hour
	}
	until := now.Add(d).UnixNano()
	if until > l.file.BlockedUntil {
		l.file.BlockedUntil = until
	}
	_ = l.save()
}

func (l *Limiter) fileBlocked(now time.Time) time.Time {
	if l.file.BlockedUntil == 0 {
		return time.Time{}
	}
	until := time.Unix(0, l.file.BlockedUntil)
	if !now.Before(until) {
		return time.Time{}
	}
	return until
}

func (l *Limiter) prune(now time.Time) {
	cutoff := now.Add(-24 * time.Hour).UnixNano()
	kept := l.file.Hits[:0]
	for _, h := range l.file.Hits {
		if h > cutoff {
			kept = append(kept, h)
		}
	}
	l.file.Hits = kept
}

func (l *Limiter) room(now time.Time) (time.Duration, bool) {
	checks := []struct {
		window time.Duration
		max    int
	}{
		{time.Minute, perMinute},
		{time.Hour, perHour},
		{24 * time.Hour, perDay},
	}
	for _, check := range checks {
		cutoff := now.Add(-check.window).UnixNano()
		n := 0
		var oldest int64
		for _, h := range l.file.Hits {
			if h <= cutoff {
				continue
			}
			n++
			if oldest == 0 || h < oldest {
				oldest = h
			}
		}
		if n >= check.max {
			wait := time.Unix(0, oldest).Add(check.window).Sub(now)
			if wait < time.Second {
				wait = time.Second
			}
			return wait, false
		}
	}
	return 0, true
}

func (l *Limiter) save() error {
	if l.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(l.file)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(l.path), "esv-usage.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, l.path)
}
