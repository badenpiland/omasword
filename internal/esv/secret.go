package esv

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "omasword"
	keyringUser    = "esv-api-key"
	keyCmdTimeout  = 30 * time.Second
)

// ErrNoKeyring is returned when the key cannot be saved or deleted because
// the OS secret service is missing or locked. The error text never includes
// the key.
var ErrNoKeyring = errors.New("no system keyring is available. Unlock gnome-keyring or KeePassXC (Secret Service), or set esv_api_key_cmd in config.json to a command that prints the key, such as \"pass show esv\"")

// Notices shown once after a legacy config.json key is scrubbed.
const (
	MigrationOKNotice     = "ESV API key moved from config.json into the system keyring."
	MigrationFailedNotice = "ESV key removed from config.json. No keyring was available; press t then a to set it again, or set esv_api_key_cmd."
)

// SaveKey stores key in the OS keyring. It does not write the key to disk.
// An empty key deletes the keyring entry.
func SaveKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return DeleteKey()
	}
	if err := keyring.Set(keyringService, keyringUser, key); err != nil {
		return redact(ErrNoKeyring, key)
	}
	return nil
}

// DeleteKey removes the keyring entry. A missing entry is success.
func DeleteKey() error {
	err := keyring.Delete(keyringService, keyringUser)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return ErrNoKeyring
}

// Resolve returns the ESV key for this process and nothing else.
//
// Order: OMASWORD_ESV_API_KEY when it is set (a one-shot value for CI or a
// single command; never written anywhere), then the OS keyring, then the
// stdout of keyCmd. keyCmd is a user-configured command such as
// "pass show esv". Its output is kept in memory only.
func Resolve(keyCmd string) string {
	if v := strings.TrimSpace(os.Getenv("OMASWORD_ESV_API_KEY")); v != "" {
		return v
	}
	if v, err := keyring.Get(keyringService, keyringUser); err == nil {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	v, err := runKeyCmd(keyCmd)
	if err != nil {
		return ""
	}
	return v
}

func runKeyCmd(cmd string) (string, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", errors.New("no key command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), keyCmdTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Stdin = nil
	if err := c.Run(); err != nil {
		// stdout is the secret. Do not attach either stream to the error.
		return "", errors.New("esv_api_key_cmd failed")
	}
	line, _, _ := strings.Cut(stdout.String(), "\n")
	return strings.TrimSpace(line), nil
}

func redact(err error, key string) error {
	if err == nil || key == "" || !strings.Contains(err.Error(), key) {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), key, "[redacted]"))
}
