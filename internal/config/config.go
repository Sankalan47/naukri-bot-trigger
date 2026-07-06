// Package config loads and validates the process-wide environment variables
// (TELEGRAM_BOT_TOKEN, HEADLESS, CHROMIUM_PATH, PLAYWRIGHT_NODEJS_PATH,
// USERS_FILE) plus the per-user Naukri accounts listed in users.json.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// UserConfig is one Telegram user's Naukri account, as configured manually by
// the maintainer in users.json.
type UserConfig struct {
	TelegramUserID int64  `json:"telegram_user_id"`
	NaukriEmail    string `json:"naukri_email"`
	NaukriPassword string `json:"naukri_password"`
	// SessionDir holds this user's auth_state.json/cookies.json/debug
	// screenshot. Defaults to sessions/<telegram_user_id> when empty.
	SessionDir string `json:"session_dir"`
}

type Config struct {
	TelegramBotToken string
	Users            []UserConfig
}

// Load reads TELEGRAM_BOT_TOKEN from the environment and the per-user roster
// from the file named by USERS_FILE (default users.json).
func Load() (*Config, error) {
	token, ok := os.LookupEnv("TELEGRAM_BOT_TOKEN")
	if !ok || token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}

	usersFile := os.Getenv("USERS_FILE")
	if usersFile == "" {
		usersFile = "users.json"
	}

	users, err := loadUsers(usersFile)
	if err != nil {
		return nil, err
	}

	return &Config{TelegramBotToken: token, Users: users}, nil
}

func loadUsers(path string) ([]UserConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var users []UserConfig
	if err := json.Unmarshal(raw, &users); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("%s must list at least one user", path)
	}

	seenID := make(map[int64]bool, len(users))
	seenDir := make(map[string]bool, len(users))
	for i := range users {
		u := &users[i]
		if u.SessionDir == "" {
			u.SessionDir = filepath.Join("sessions", strconv.FormatInt(u.TelegramUserID, 10))
		}
		if seenID[u.TelegramUserID] {
			return nil, fmt.Errorf("%s: duplicate telegram_user_id %d", path, u.TelegramUserID)
		}
		seenID[u.TelegramUserID] = true
		if seenDir[u.SessionDir] {
			return nil, fmt.Errorf("%s: duplicate session_dir %q", path, u.SessionDir)
		}
		seenDir[u.SessionDir] = true
	}
	return users, nil
}
