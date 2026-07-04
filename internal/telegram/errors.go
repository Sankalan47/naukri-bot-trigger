package telegram

import (
	"errors"
	"fmt"

	"github.com/sankalan47/naukri-bot/internal/naukri"
)

// errorMessage translates a naukri.Client error into user-facing Telegram
// copy, mirroring bot.py's _error_msg. This is presentation logic and
// deliberately kept out of the naukri package, which stays Telegram-agnostic.
func errorMessage(err error) string {
	var nerr *naukri.Error
	if errors.As(err, &nerr) {
		switch nerr.Kind {
		case naukri.KindSessionExpired:
			return "Session expired. Use /refresh_session to re-authenticate."
		case naukri.KindSelector:
			return fmt.Sprintf("Naukri UI may have changed — check server logs.\nDetails: %s", nerr.Message)
		case naukri.KindTimeout:
			return "Naukri.com didn't respond in time. Try again."
		}
	}
	return fmt.Sprintf("Something went wrong: %v", err)
}
