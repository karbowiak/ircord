package irc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
	"ircord/backend/internal/storage"
)

var ErrStorage = errors.New("application storage is unavailable")
var ErrUncertain = errors.New("IRC acceptance is uncertain; reconcile message history before sending this command again")

func (w *workspace) commandUncertain(clientID, message string) {
	if w.store != nil {
		_ = w.store.UncertainCommand(w.ownerID, clientID, message)
	}
	w.commandError(clientID, message)
}

func (w *workspace) load() {
	if w.store == nil {
		return
	}
	entities, err := w.store.Entities(w.ownerID)
	if err != nil {
		w.storageErr = err
		return
	}
	invites, err := w.store.Invitations(w.ownerID)
	if err != nil {
		w.storageErr = err
		return
	}
	for _, v := range invites {
		w.invitations[v.ID] = v
	}
	for kind, items := range entities {
		for _, raw := range items {
			switch kind {
			case "connection":
				var info ConnectionInfo
				err = json.Unmarshal(raw, &info)
				if err == nil {
					info.State = "disconnected"
					w.info[info.NetworkID] = info
				}
			case "member":
				var v Member
				err = json.Unmarshal(raw, &v)
				if err == nil {
					if v.ID != w.ownerID {
						v.Status = "offline"
					}
					w.members[v.ID] = v
					if v.ID == w.ownerID {
						normalizeSelfName(&v)
						w.self = v
						w.members[v.ID] = v
					}
				}
			case "network":
				var v Network
				err = json.Unmarshal(raw, &v)
				if err == nil {
					v.Status = "disconnected"
					v.Account = ""
					v.Features = model.Features{Metadata: []string{}}
					v.Capabilities = []string{}
					w.networks[v.ID] = v
				}
			case "conversation":
				var v Conversation
				err = json.Unmarshal(raw, &v)
				if err == nil {
					// Channel previews follow the shared channel settings, applied when the
					// channel is joined; an old per-account value must not linger.
					clearChannelSettings(&v)
					w.conversations[v.ID] = v
					w.messages[v.ID] = []Message{}
				}
			case "preferences":
				err = json.Unmarshal(raw, &w.preferences)
			}
			if err != nil {
				w.storageErr = err
				return
			}
		}
	}
	networks, err := w.store.Networks(w.ownerID)
	if err != nil {
		w.storageErr = err
		return
	}
	for _, v := range networks {
		w.configs[v.Config.ID] = v.Config
	}
	rules, ruleErr := w.store.AppData(w.ownerID, "notification-settings.updated")
	if ruleErr != nil {
		w.storageErr = ruleErr
		return
	}
	w.policy = notify.Compile(rules)
	w.seq, err = w.store.Head(w.ownerID)
	w.storageErr = err
}
func (m *Manager) StorageError(owner string) error {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return ErrNotFound
	}
	if w.storageErr != nil {
		return ErrStorage
	}
	return nil
}
func (m *Manager) DropOwner(owner string) error {
	m.mu.Lock()
	m.deletedOwners[owner] = true
	w := m.owners[owner]
	m.mu.Unlock()
	if w == nil {
		return nil
	}
	w.mu.Lock()
	w.deleted = true
	connections := []*connection{}
	for _, c := range w.connections {
		connections = append(connections, c)
	}
	w.mu.Unlock()
	for _, c := range connections {
		c.mu.Lock()
		c.ready = false
		c.mu.Unlock()
		c.cancel()
		c.closeTransport()
	}
	for _, c := range connections {
		<-c.done
		c.workers.Wait()
	}
	w.mu.Lock()
	w.closeTraceSubscribers()
	for ch := range w.subscribers {
		close(ch)
		delete(w.subscribers, ch)
	}
	remaining := len(w.connections)
	w.connections = map[string]*connection{}
	w.mu.Unlock()
	m.mu.Lock()
	if m.owners[owner] == w {
		m.connectionCount -= remaining
		delete(m.owners, owner)
	}
	m.mu.Unlock()
	return nil
}
func (m *Manager) Restore() error {
	if m.options.Store == nil {
		return nil
	}
	saved, err := m.options.Store.Networks("")
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, n := range saved {
		if !seen[n.Owner] {
			if err = m.options.Store.UncertainCommands(n.Owner); err != nil {
				return err
			}
			seen[n.Owner] = true
		}
		w := m.space(n.Owner)
		w.mu.Lock()
		loadErr := w.storageErr
		w.mu.Unlock()
		if loadErr != nil {
			return loadErr
		}
		if n.Enabled {
			if _, err = m.Connect(n.Owner, n.Config); err != nil {
				return fmt.Errorf("restore network: %w", err)
			}
		}
	}
	return nil
}
func (c *connection) saveTargets() error {
	w := c.owner
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.deleted {
		return ErrNotFound
	}
	cfg := clone(w.configs[c.config.ID])
	c.mu.Lock()
	cfg.Channels = orderedTargets(c.desired, cfg.Channels)
	c.mu.Unlock()
	cfg.AutoJoin = append([]string{}, cfg.Channels...)
	if w.store != nil {
		if err := w.store.SaveNetwork(w.ownerID, cfg, true); err != nil {
			return err
		}
	}
	w.configs[cfg.ID] = cfg
	return nil
}
func orderedTargets(desired map[string]string, previous []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, entry := range previous {
		parts := strings.Fields(entry)
		if len(parts) == 0 {
			continue
		}
		key := strings.ToLower(parts[0])
		if value, ok := desired[key]; ok && !seen[key] {
			result = append(result, value)
			seen[key] = true
		}
	}
	extra := []string{}
	for key, value := range desired {
		if !seen[key] {
			extra = append(extra, value)
		}
	}
	sort.Strings(extra)
	return append(result, extra...)
}
func (m *Manager) SaveAppData(owner, kind string, data json.RawMessage) (json.RawMessage, error) {
	if kind != "appearance.updated" && kind != "library.updated" && kind != "client-preferences.updated" && kind != "notification-settings.updated" {
		return nil, fmt.Errorf("invalid app data kind")
	}
	if m.options.Store == nil {
		return nil, ErrUnsupported
	}
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emit(kind, data)
	if kind == "notification-settings.updated" {
		for key, conv := range w.conversations {
			conv = w.refreshUnread(conv)
			w.conversations[key] = conv
			w.emit("conversation.updated", conv)
		}
	}
	if w.storageErr != nil {
		return nil, ErrStorage
	}
	return data, nil
}
func (m *Manager) AppData(owner, kind string) (json.RawMessage, error) {
	if m.options.Store == nil {
		return nil, nil
	}
	return m.options.Store.AppData(owner, kind)
}
func (m *Manager) command(owner, client string) (storage.Command, error) {
	if m.options.Store == nil {
		return storage.Command{}, ErrUnsupported
	}
	return m.options.Store.Command(owner, client)
}
func (m *Manager) Command(owner, client string) (storage.Command, error) {
	return m.command(owner, client)
}
func (c *connection) lookupMessage(conversation, message string) (Message, bool) {
	for _, msg := range c.owner.messages[conversation] {
		if msg.ID == message || msg.UpstreamID == message {
			return clone(msg), true
		}
	}
	if c.owner.store != nil {
		msg, err := c.owner.store.Message(c.owner.ownerID, conversation, message)
		if err == nil {
			return msg, true
		}
	}
	return Message{}, false
}
func (c *connection) restoreNicks() {
	for _, v := range c.owner.members {
		if strings.HasPrefix(v.ID, c.config.ID+":") {
			c.nicks[c.fold(v.Nick)] = v.ID
		}
	}
}

var ErrConflict = errors.New("saved data changed on another device; reload and retry")

func AppRevision(data json.RawMessage) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (m *Manager) SaveAppDataCAS(owner, kind, revision string, data json.RawMessage) (json.RawMessage, error) {
	if kind != "appearance.updated" && kind != "library.updated" && kind != "client-preferences.updated" && kind != "notification-settings.updated" {
		return nil, fmt.Errorf("invalid app data kind")
	}
	if len(data) > 4<<20 || !json.Valid(data) {
		return nil, fmt.Errorf("invalid saved data")
	}
	if m.options.Store == nil {
		return nil, ErrUnsupported
	}
	canonical, err := json.Marshal(data)
	if err != nil || len(canonical) > 4<<20 {
		return nil, fmt.Errorf("invalid saved data")
	}
	data = canonical
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	saved, err := w.store.AppData(owner, kind)
	if err != nil {
		return nil, ErrStorage
	}
	if AppRevision(saved) != revision {
		return nil, ErrConflict
	}
	w.emit(kind, data)
	if w.storageErr != nil {
		return nil, ErrStorage
	}
	return data, nil
}

// Persistent history is authoritative; this cache only accelerates lookups.
func (w *workspace) boundCache(current string) {
	if w.store == nil {
		return
	}
	w.cacheClock++
	w.cacheTouched[current] = w.cacheClock
	total := 0
	for _, items := range w.messages {
		total += len(items)
	}
	for total > 5000 {
		oldest := ""
		var stamp uint64
		for key, items := range w.messages {
			if key == current || len(items) == 0 {
				continue
			}
			at := w.cacheTouched[key]
			if oldest == "" || at < stamp {
				oldest, stamp = key, at
			}
		}
		if oldest == "" {
			break
		}
		total -= len(w.messages[oldest])
		delete(w.messages, oldest)
		delete(w.cacheTouched, oldest)
	}
}
