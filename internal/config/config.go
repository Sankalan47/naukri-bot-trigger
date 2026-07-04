// Package config loads and validates environment variables shared by the
// bot and dev tooling, matching the .env-based setup used by the Python
// version (TELEGRAM_BOT_TOKEN, ALLOWED_USER_ID, NAUKRI_EMAIL, NAUKRI_PASSWORD).
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	TelegramBotToken string
	AllowedUserID    int64
	NaukriEmail      string
	NaukriPassword   string
}

// Load reads required and optional environment variables into a Config.
// TELEGRAM_BOT_TOKEN and ALLOWED_USER_ID are required and fail fast if
// missing, matching Python's os.environ["X"] KeyError behavior.
// NAUKRI_EMAIL/NAUKRI_PASSWORD remain optional, as in the Python os.environ.get(...) calls.
func Load() (*Config, error) {
	token, ok := os.LookupEnv("TELEGRAM_BOT_TOKEN")
	if !ok || token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}

	rawUserID, ok := os.LookupEnv("ALLOWED_USER_ID")
	if !ok || rawUserID == "" {
		return nil, fmt.Errorf("ALLOWED_USER_ID not set")
	}
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ALLOWED_USER_ID must be an integer: %w", err)
	}

	return &Config{
		TelegramBotToken: token,
		AllowedUserID:    userID,
		NaukriEmail:      os.Getenv("NAUKRI_EMAIL"),
		NaukriPassword:   os.Getenv("NAUKRI_PASSWORD"),
	}, nil
}
