package telegram

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/sankalan47/naukri-bot/internal/naukri"
)

func (b *Bot) replyText(chatID int64, text string) (tgbotapi.Message, error) {
	return b.api.Send(tgbotapi.NewMessage(chatID, text))
}

func (b *Bot) editText(chatID int64, messageID int, text string) {
	if _, err := b.api.Send(tgbotapi.NewEditMessageText(chatID, messageID, text)); err != nil {
		log.Printf("failed to edit message: %v", err)
	}
}

func (b *Bot) cmdStart(chatID int64) {
	b.replyText(chatID,
		"Naukri Bot ready.\n\n"+
			"/update_bio — Update profile summary\n"+
			"/update_headline — Update profile headline\n"+
			"/add_skill — Add a skill\n"+
			"/remove_skill — Remove a skill\n"+
			"/status — Check session health\n"+
			"/refresh_session — Instructions to renew session\n"+
			"/cancel — Cancel current operation",
	)
}

func (b *Bot) cmdStatus(chatID int64, client *naukri.Client) {
	msg, err := b.replyText(chatID, "Checking session...")
	if err != nil {
		log.Printf("failed to send status message: %v", err)
		return
	}
	result := client.CheckStatus()
	if result.LoggedIn {
		b.editText(chatID, msg.MessageID, fmt.Sprintf("Session active. Profile: %s", result.ProfileName))
	} else {
		b.editText(chatID, msg.MessageID, fmt.Sprintf("Session inactive: %s\n\nUse /refresh_session.", result.Reason))
	}
}

func (b *Bot) cmdRefreshSession(chatID int64) {
	b.replyText(chatID,
		"To refresh your Naukri session:\n\n"+
			"Option A — Credentials (recommended):\n"+
			"  Ensure NAUKRI_EMAIL and NAUKRI_PASSWORD are set in .env.\n"+
			"  The bot re-logs in automatically when the session expires.\n\n"+
			"Option B — Manual cookie export:\n"+
			"  1. On your local machine run:\n"+
			"     ./bin/save-session --save-cookies\n"+
			"  2. Log in to Naukri in the browser window that opens.\n"+
			"  3. SCP the saved file to the VM:\n"+
			"     scp auth_state.json user@vm-ip:~/naukri-bot/\n"+
			"  4. Restart the bot: sudo systemctl restart naukri-bot",
	)
}

func (b *Bot) enterConversation(chatID int64, kind ConvKind, prompt string) {
	b.conv.Enter(chatID, kind)
	b.replyText(chatID, prompt)
}

func (b *Bot) receiveBio(chatID int64, client *naukri.Client, text string) {
	msg, err := b.replyText(chatID, "Updating bio...")
	if err != nil {
		log.Printf("failed to send updating message: %v", err)
		return
	}
	if err := client.UpdateBio(text); err != nil {
		b.editText(chatID, msg.MessageID, errorMessage(err))
		return
	}
	b.editText(chatID, msg.MessageID, "Bio updated successfully.")
}

func (b *Bot) receiveHeadline(chatID int64, client *naukri.Client, text string) {
	msg, err := b.replyText(chatID, "Updating headline...")
	if err != nil {
		log.Printf("failed to send updating message: %v", err)
		return
	}
	if err := client.UpdateHeadline(text); err != nil {
		b.editText(chatID, msg.MessageID, errorMessage(err))
		return
	}
	b.editText(chatID, msg.MessageID, "Headline updated successfully.")
}

func (b *Bot) receiveAddSkill(chatID int64, client *naukri.Client, skill string) {
	msg, err := b.replyText(chatID, fmt.Sprintf("Adding skill '%s'...", skill))
	if err != nil {
		log.Printf("failed to send adding-skill message: %v", err)
		return
	}
	added, err := client.AddSkill(skill)
	if err != nil {
		b.editText(chatID, msg.MessageID, errorMessage(err))
		return
	}
	if added {
		b.editText(chatID, msg.MessageID, fmt.Sprintf("Added '%s' to your Naukri skills.", skill))
	} else {
		b.editText(chatID, msg.MessageID, fmt.Sprintf("'%s' is already in your skills list.", skill))
	}
}

func (b *Bot) receiveRemoveSkill(chatID int64, client *naukri.Client, skill string) {
	msg, err := b.replyText(chatID, fmt.Sprintf("Removing skill '%s'...", skill))
	if err != nil {
		log.Printf("failed to send removing-skill message: %v", err)
		return
	}
	removed, err := client.RemoveSkill(skill)
	if err != nil {
		b.editText(chatID, msg.MessageID, errorMessage(err))
		return
	}
	if removed {
		b.editText(chatID, msg.MessageID, fmt.Sprintf("Removed '%s' from your Naukri skills.", skill))
	} else {
		b.editText(chatID, msg.MessageID, fmt.Sprintf("'%s' was not found in your skills list.", skill))
	}
}
