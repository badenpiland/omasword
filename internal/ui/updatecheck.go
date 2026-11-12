package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"omasword/internal/settings"
	"omasword/internal/update"
)

// updateAvailableMsg carries a newer release tag from the daily check.
type updateAvailableMsg struct{ tag string }

// findUpdate is the daily check. Tests replace it so they never reach
// GitHub.
var findUpdate = func() string {
	cfg, _ := settings.Load()
	if update.OptedOut(cfg.UpdateCheck != nil && !*cfg.UpdateCheck) {
		return ""
	}
	return update.New().Notice(context.Background(), update.DefaultCacheDir(), time.Now())
}

// checkForUpdate runs the check as a Bubble Tea command, off the UI
// thread, so a slow or missing network never holds up startup.
func checkForUpdate() tea.Cmd {
	return func() tea.Msg {
		if tag := findUpdate(); tag != "" {
			return updateAvailableMsg{tag: tag}
		}
		return nil
	}
}
