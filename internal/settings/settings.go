package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	SelectedTranslation string `json:"selected_translation"`
	CurrentBook         int    `json:"current_book"`
	CurrentChapter      int    `json:"current_chapter"`
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

	return s, nil
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

	return os.WriteFile(path, data, 0o644)
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
	_ = os.WriteFile(path, data, 0o644)
	return data, nil
}
