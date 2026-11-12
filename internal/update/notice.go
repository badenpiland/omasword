package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// CheckInterval is how often the reader may ask GitHub for a new
	// release. Failed checks count too, so being offline costs one
	// attempt a day.
	CheckInterval = 24 * time.Hour
	// NoticeTimeout bounds the reader's background check.
	NoticeTimeout = 10 * time.Second
	// OptOutEnv turns the reader's check off for one process when set to
	// anything but "" or "0". `update_check: false` in config.json turns
	// it off for good.
	OptOutEnv = "OMASWORD_NO_UPDATE_CHECK"

	checkFile = "update-check.json"
)

// checkRecord is the cache file. It holds no personal data: when the last
// check ran and the newest tag it saw.
type checkRecord struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest,omitempty"`
}

// DefaultCacheDir is ~/.cache/omasword (or $XDG_CACHE_HOME/omasword).
func DefaultCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "omasword")
}

// OptedOut reports whether the reader's update check is off, from the
// config setting or OptOutEnv.
func OptedOut(disabledInConfig bool) bool {
	v := strings.TrimSpace(os.Getenv(OptOutEnv))
	return disabledInConfig || (v != "" && v != "0")
}

// Notice returns the newer release tag the reader should mention, or "".
// It asks GitHub at most once per CheckInterval, remembering the answer in
// cacheDir; otherwise it uses the remembered tag. Errors are swallowed: a
// failed check just means no notice. It is meant to run off the UI thread.
func (u *Updater) Notice(ctx context.Context, cacheDir string, now time.Time) string {
	if cacheDir == "" {
		return "" // nowhere to remember the check, so don't make one
	}
	if exe, err := Executable(); err == nil && strings.HasPrefix(exe, packageManaged) {
		return "" // a system package; `omasword update` would refuse it
	}
	path := filepath.Join(cacheDir, checkFile)
	rec := readRecord(path)
	if rec.CheckedAt.IsZero() || now.Sub(rec.CheckedAt) >= CheckInterval || rec.CheckedAt.After(now) {
		// Record the attempt first, so a crash or a hung request still
		// counts as today's check.
		rec.CheckedAt = now
		if writeRecord(cacheDir, path, rec) != nil {
			return ""
		}
		ctx, cancel := context.WithTimeout(ctx, NoticeTimeout)
		defer cancel()
		if rel, err := u.Latest(ctx); err == nil {
			rec.Latest = rel.Tag
			_ = writeRecord(cacheDir, path, rec)
		}
	}
	if rec.Latest == "" {
		return ""
	}
	if newer, err := Newer(rec.Latest, u.Current); err != nil || !newer {
		return ""
	}
	return rec.Latest
}

func readRecord(path string) checkRecord {
	var rec checkRecord
	data, err := os.ReadFile(path) // #nosec G304 -- fixed name under the user's cache dir
	if err != nil || json.Unmarshal(data, &rec) != nil {
		return checkRecord{}
	}
	if _, err := parseVersion(rec.Latest); err != nil {
		rec.Latest = "" // only ever show a plain version
	}
	return rec
}

// writeRecord writes via a temp file and rename, like the other caches.
func writeRecord(dir, path string, rec checkRecord) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".update-check-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}
