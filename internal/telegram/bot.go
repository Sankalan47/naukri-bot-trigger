// Package telegram wires a whitelisted Telegram chat to a naukri.Client,
// replacing bot.py's python-telegram-bot Application. go-telegram-bot-api/v5
// hands back a raw update channel with no handler registry, so this package
// dispatches every update itself and hand-rolls the multi-step conversation
// flows (see conversation.go) that python-telegram-bot's ConversationHandler
// provided for free.
package telegram

import (
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/sankalan47/naukri-bot/internal/naukri"
)

type Bot struct {
	api           *tgbotapi.BotAPI
	naukri        *naukri.Client
	allowedUserID int64
	conv          *Manager
}

func New(api *tgbotapi.BotAPI, naukriClient *naukri.Client, allowedUserID int64) *Bot {
	b := &Bot{api: api, naukri: naukriClient, allowedUserID: allowedUserID}
	b.conv = newManager(b.onConversationTimeout)
	return b
}

func (b *Bot) onConversationTimeout(chatID int64) {
	b.replyText(chatID, "Conversation timed out. Send the command again.")
}

// Notify sends a plain text message to chatID, outside the normal command
// dispatch flow. Used by the scheduler to report scheduled job results.
func (b *Bot) Notify(chatID int64, text string) {
	if _, err := b.replyText(chatID, text); err != nil {
		log.Printf("failed to send scheduled notification: %v", err)
	}
}

// Run polls for updates and dispatches them until the updates channel closes
// (StopReceivingUpdates was called, e.g. during shutdown).
func (b *Bot) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	log.Println("Bot polling started")
	for update := range updates {
		go b.dispatch(update)
	}
}

// Stop stops the update poller.
func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
}

func (b *Bot) dispatch(update tgbotapi.Update) {
	msg := update.Message
	if msg == nil || msg.From == nil || msg.From.ID != b.allowedUserID {
		return // silently ignore updates from anyone but the whitelisted user
	}
	chatID := msg.Chat.ID

	if !msg.IsCommand() {
		if kind, ok := b.conv.Get(chatID); ok {
			b.handleConversationText(chatID, kind, msg.Text)
		}
		return
	}

	// Any command interrupts an active conversation, then (if it's one of
	// the 4 conversation-starting commands) begins a new one.
	b.conv.Clear(chatID)

	switch msg.Command() {
	case "start":
		b.cmdStart(chatID)
	case "status":
		b.cmdStatus(chatID)
	case "refresh_session":
		b.cmdRefreshSession(chatID)
	case "cancel":
		b.replyText(chatID, "Cancelled.")
	case "update_bio":
		b.enterConversation(chatID, AwaitBio, "Send me the new profile summary text (or /cancel to abort):")
	case "update_headline":
		b.enterConversation(chatID, AwaitHeadline, "Send me the new headline text (or /cancel to abort):")
	case "add_skill":
		b.enterConversation(chatID, AwaitSkillAdd, "Send me the skill name to add (or /cancel to abort):")
	case "remove_skill":
		b.enterConversation(chatID, AwaitSkillRemove, "Send me the skill name to remove (or /cancel to abort):")
	}
}

func (b *Bot) handleConversationText(chatID int64, kind ConvKind, text string) {
	trimmed := strings.TrimSpace(text)

	if trimmed == "" {
		b.replyText(chatID, emptyPrompt(kind))
		b.conv.Enter(chatID, kind) // empty input re-prompts and stays in the same state
		return
	}

	b.conv.Clear(chatID)
	switch kind {
	case AwaitBio:
		b.receiveBio(chatID, trimmed)
	case AwaitHeadline:
		b.receiveHeadline(chatID, trimmed)
	case AwaitSkillAdd:
		b.receiveAddSkill(chatID, trimmed)
	case AwaitSkillRemove:
		b.receiveRemoveSkill(chatID, trimmed)
	}
}

func emptyPrompt(kind ConvKind) string {
	switch kind {
	case AwaitSkillAdd, AwaitSkillRemove:
		return "Skill name cannot be empty. Try again or /cancel."
	default:
		return "Text cannot be empty. Try again or /cancel."
	}
}
