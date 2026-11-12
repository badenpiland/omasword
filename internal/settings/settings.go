package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Settings struct {
	SelectedTranslation string `json:"selected_translation"`
	CurrentBook         int    `json:"current_book"`
	CurrentChapter      int    `json:"current_chapter"`
	// ESVAPIKeyCmd is a command whose first line of stdout is the ESV API
	// key (for example "pass show esv"). The command is stored here. Its
	// output is not.
	ESVAPIKeyCmd string `json:"esv_api_key_cmd,omitempty"`
	// ComparisonColumns is how many translations the comparison view
	// shows side by side (2 to 4). 0 means the default, 2.
	ComparisonColumns int `json:"comparison_columns,omitempty"`
	// UpdateCheck set to false stops the reader's once-a-day check for a
	// newer release. Unset means on.
	UpdateCheck *bool `json:"update_check,omitempty"`
}

// legacyDirName is the config directory used before the rename from
// sword-tui. Its settings are copied over the first time omasword runs.
const legacyDirName = "sword-tui"

func configPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(configDir, "omasword")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

func Load() (Settings, error) {
	var s Settings

	path, err := configPath()
	if err != nil {
		return s, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data, err = migrateLegacy(path)
	}
	if err != nil {
		// No config = just return zero value, no error
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}

	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, &UnreadableError{Path: path, Err: err}
	}
	return s, nil
}

// UnreadableError means config.json exists but isn't valid JSON (a hand
// edit gone wrong, say). The next Save keeps a copy of it first, so
// settings such as esv_api_key_cmd can be copied back.
type UnreadableError struct {
	Path string
	Err  error
}

func (e *UnreadableError) Error() string {
	return fmt.Sprintf("can't read %s: %v", e.Path, e.Err)
}

func (e *UnreadableError) Unwrap() error { return e.Err }

// IsUnreadable reports whether err is an UnreadableError.
func IsUnreadable(err error) bool {
	var u *UnreadableError
	return errors.As(err, &u)
}

func Save(s Settings) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := keepUnreadable(path); err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

// keepUnreadable renames a config.json that isn't valid JSON to
// config.json.<time>.bad before Save replaces it, so a typo doesn't cost
// the settings in it. If the copy can't be made, Save fails and the file
// stays as it is.
func keepUnreadable(path string) error {
	old, err := os.ReadFile(path)
	if err != nil || json.Valid(old) {
		return nil // nothing there, or nothing to lose
	}
	return os.Rename(path, BackupPath(path, time.Now()))
}

// BackupPath is where Save keeps an unreadable config.json.
func BackupPath(path string, now time.Time) string {
	return path + "." + now.Format("20060102-150405") + ".bad"
}

// writeFileAtomic writes data to a temp file beside path and renames it into
// place, so a crash or kill mid-write never leaves a truncated config.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// migrateLegacy copies a sword-tui config.json to path, leaving the original
// in place so sword-tui keeps working if it's still installed.
func migrateLegacy(path string) ([]byte, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(configDir, legacyDirName, "config.json"))
	if err != nil {
		return nil, err
	}
	_ = writeFileAtomic(path, data, 0o644)
	return data, nil
}
