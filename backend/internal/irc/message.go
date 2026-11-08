package irc

import (
	"errors"

	"ircord/backend/internal/storage"
)

// Message reads an account's retained message without fetching upstream history.
// A conversation removed from its workspace is no longer an accessible source.
func (m *Manager) Message(owner, conversation, id string) (Message, error) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return Message{}, ErrNotFound
	}
	if _, ok := w.conversations[conversation]; !ok {
		return Message{}, ErrNotFound
	}
	var msg *Message
	if w.store != nil {
		stored, err := w.store.Message(owner, conversation, id)
		if errors.Is(err, storage.ErrNotFound) {
			return Message{}, ErrNotFound
		}
		if err != nil {
			return Message{}, ErrStorage
		}
		msg = &stored
	} else {
		msg = w.memoryMessage(conversation, id)
	}
	if msg == nil || msg.ID != id || !savable(*msg) {
		return Message{}, ErrNotFound
	}
	return clone(*msg), nil
}
