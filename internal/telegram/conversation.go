package telegram

import (
	"sync"
	"time"
)

// ConvKind identifies which multi-step command a chat is currently in the
// middle of, replacing python-telegram-bot's ConversationHandler states
// (AWAIT_BIO, AWAIT_SKILL, AWAIT_RM_SKILL, AWAIT_HEADLINE).
type ConvKind int

const (
	AwaitBio ConvKind = iota
	AwaitHeadline
	AwaitSkillAdd
	AwaitSkillRemove
)

// conversationTimeout matches python-telegram-bot's conversation_timeout=120.
const conversationTimeout = 120 * time.Second

type convState struct {
	kind  ConvKind
	timer *time.Timer
}

// Manager tracks at most one active conversation per chat. go-telegram-bot-api
// hands back a raw update channel with no handler/FSM registry, so this
// replaces ConversationHandler's per-chat state tracking and timeout.
type Manager struct {
	mu        sync.Mutex
	states    map[int64]*convState
	onTimeout func(chatID int64)
}

func newManager(onTimeout func(chatID int64)) *Manager {
	return &Manager{states: make(map[int64]*convState), onTimeout: onTimeout}
}

// Enter starts (or restarts, e.g. after an empty-text re-prompt) a
// conversation for chatID, arming a fresh 120s timeout. Any previously
// active conversation for that chat is replaced.
func (m *Manager) Enter(chatID int64, kind ConvKind) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.states[chatID]; ok {
		existing.timer.Stop()
	}

	state := &convState{kind: kind}
	state.timer = time.AfterFunc(conversationTimeout, func() {
		m.expire(chatID, state)
	})
	m.states[chatID] = state
}

// expire fires when a conversation's timer elapses. It guards against a
// stale timer acting after the conversation already completed or was
// replaced by comparing against the state pointer captured at Enter time.
func (m *Manager) expire(chatID int64, state *convState) {
	m.mu.Lock()
	current, ok := m.states[chatID]
	if !ok || current != state {
		m.mu.Unlock()
		return
	}
	delete(m.states, chatID)
	m.mu.Unlock()

	if m.onTimeout != nil {
		m.onTimeout(chatID)
	}
}

// Get returns the active conversation kind for chatID, if any.
func (m *Manager) Get(chatID int64) (ConvKind, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.states[chatID]
	if !ok {
		return 0, false
	}
	return state.kind, true
}

// Clear ends any active conversation for chatID (used by /cancel, by any
// other incoming command, and once a conversation's receive step runs).
func (m *Manager) Clear(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.states[chatID]; ok {
		state.timer.Stop()
		delete(m.states, chatID)
	}
}
