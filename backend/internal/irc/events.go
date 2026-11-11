package irc

import (
	"encoding/json"
	"sync"

	"ircord/backend/internal/notify"
)

// emit commits account state before publishing to its bounded live queues.
// The caller holds the workspace mutex throughout commit and publication.
func (w *workspace) emit(kind string, data any) {
	if w.manager != nil && w.manager.parties != nil {
		w.manager.parties.capture(w, kind, data)
	}
	if w.deleted {
		w.storageErr = ErrNotFound
		return
	}
	if w.storageErr != nil {
		w.commandError("", "Storage is unavailable; restart after restoring storage to resynchronize.")
		return
	}
	if member, ok := data.(Member); ok {
		member.Ignored = member.ID != w.self.ID && w.policy.Ignored(member.Nick, member.Username, member.Host)
		data = member
	}
	var e Event
	if w.store != nil {
		var err error
		e, err = w.store.Commit(w.ownerID, kind, data)
		if err != nil {
			w.storageErr = err
			w.commandError("", "Storage is unavailable; live delivery is paused.")
			return
		}
		w.seq = e.Seq
		w.storageErr = nil
	} else {
		w.seq++
		b, _ := json.Marshal(data)
		e = Event{V: 1, Seq: w.seq, Type: kind, Data: b}
	}
	if kind == "notification-settings.updated" {
		raw, _ := json.Marshal(data)
		w.policy = notify.Compile(raw)
	}
	if w.store == nil {
		w.events = append(w.events, e)
		if len(w.events) > 256 {
			w.events = append([]Event(nil), w.events[len(w.events)-256:]...)
		}
	}
	for ch := range w.subscribers {
		select {
		case ch <- clone(e):
		default:
			delete(w.subscribers, ch)
			close(ch)
		}
	}
}

func (w *workspace) commandError(clientID, message string) {
	if w.store != nil && clientID != "" {
		_ = w.store.FailCommand(w.ownerID, clientID, message)
	}
	w.sendCommandError(clientID, message)
}

func (w *workspace) sendCommandError(clientID, message string) {
	for ch := range w.subscribers {
		select {
		case ch <- Event{V: 1, Type: "command.error", ClientID: clientID, Message: message}:
		default:
			delete(w.subscribers, ch)
			close(ch)
		}
	}
}

// Subscribe atomically snapshots the replay cursor and attaches the live queue.
// A slow consumer is closed and must resynchronize, never stall another owner.
func (m *Manager) Subscribe(owner string, after uint64) ([]Event, <-chan Event, bool, func()) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	ch := make(chan Event, 64)
	if len(w.subscribers) >= 16 {
		close(ch)
		return nil, ch, false, func() {}
	}
	valid := after <= w.seq && (len(w.events) == 0 || after+1 >= w.events[0].Seq)
	replay := []Event{}
	if valid {
		for _, e := range w.events {
			if e.Seq > after {
				replay = append(replay, clone(e))
			}
		}
	}
	if w.store != nil {
		var err error
		replay, valid, err = w.store.Replay(owner, after, 4096)
		if err != nil {
			valid = false
			w.storageErr = err
		}
	}
	// Re-encode older durable DTOs so reconnect replay satisfies the current
	// schema, including default-false channel sensitivity on legacy settings.
	for i := range replay {
		switch replay[i].Type {
		case "network.updated":
			var network Network
			if json.Unmarshal(replay[i].Data, &network) != nil {
				valid = false
				break
			}
			replay[i].Data, _ = json.Marshal(network)
		case "conversation.updated":
			var conv Conversation
			if json.Unmarshal(replay[i].Data, &conv) != nil {
				valid = false
				break
			}
			replay[i].Data, _ = json.Marshal(conv)
		}
		if !valid {
			break
		}
	}
	w.subscribers[ch] = struct{}{}
	var once sync.Once
	return replay, ch, valid, func() {
		once.Do(func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			if _, ok := w.subscribers[ch]; ok {
				delete(w.subscribers, ch)
				close(ch)
			}
		})
	}
}
