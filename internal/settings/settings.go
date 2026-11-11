package settings

import (
	"encoding/json"
	"os"
	"path/filepath"

	"omasword/internal/esv"
)

type Settings struct {
	SelectedTranslation string `json:"selected_translation"`
	CurrentBook         int    `json:"current_book"`
	CurrentChapter      int    `json:"current_chapter"`
	// ESVAPIKeyCmd is a command whose first line of stdout is the ESV API
	// key (for example "pass show esv"). The command is stored here. Its
	// output is not.
	ESVAPIKeyCmd string `json:"esv_api_key_cmd,omitempty"`
	// LegacyESVAPIKey is read only so an older config can be migrated into
	// the keyring. Save always clears it. The key itself is never written.
	LegacyESVAPIKey string `json:"esv_api_key,omitempty"`
	// KeyNotice is set when a legacy key was scrubbed from this file. It
	// is not serialized.
	KeyNotice string `json:"-"`
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
		return Settings{}, err
	}
	if err := s.scrubLegacyKey(); err != nil {
		return s, err
	}
	return s, nil
}

// scrubLegacyKey moves a plaintext esv_api_key into the OS keyring and
// rewrites the file without it. The rewrite happens even when the keyring
// refuses the secret, so the key does not stay in the file.
func (s *Settings) scrubLegacyKey() error {
	if s.LegacyESVAPIKey == "" {
		return nil
	}
	secret := s.LegacyESVAPIKey
	s.LegacyESVAPIKey = ""
	if err := esv.SaveKey(secret); err != nil {
		s.KeyNotice = esv.MigrationFailedNotice
	} else {
		s.KeyNotice = esv.MigrationOKNotice
	}
	return Save(*s)
}

func Save(s Settings) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	// A key must never be written, even if a caller left the legacy field set.
	s.LegacyESVAPIKey = ""
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
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
