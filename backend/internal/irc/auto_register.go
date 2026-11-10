package irc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
)

// Channels you create on the home network are registered to your account with ChanServ, so
// they stay yours after everyone leaves, much like a Discord server (#65). "Create" means a
// JOIN the user asked for whose NAMES reply has only them, as an operator. Joins IRCord
// replays on reconnect never register anything, and other networks are never touched.

// creatingWindow bounds how long after the user's JOIN its NAMES reply still counts.
const creatingWindow = 30 * time.Second

func (c *connection) home() bool {
	if c.manager == nil {
		return false
	}
	home := c.manager.options.Home
	return home.ID != "" && c.config.ID == home.ID && c.manager.partyServer(c.config)
}

// markCreating notes a join the user asked for, so its NAMES reply is checked.
func (c *connection) markCreating(target string) {
	if c.manager != nil && c.manager.options.DisableAutoRegistration {
		return
	}
	if c.owner.ownerID == partyControlOwner {
		return
	}
	if !c.home() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.explicitCreating[c.foldUnlocked(target)] {
		return
	}
	if c.creating == nil {
		c.creating = map[string]time.Time{}
	}
	for key, at := range c.creating {
		if time.Since(at) > creatingWindow {
			delete(c.creating, key)
		}
	}
	if len(c.creating) < 64 {
		c.creating[girc.ToRFC1459(target)] = time.Now()
	}
}

// createdChannel runs on the NAMES reply (366) and starts registration when the user just
// created the channel.
func (c *connection) createdChannel(client *girc.Client, e girc.Event) {
	if e.Command != "366" || len(e.Params) < 2 || !girc.IsValidChannel(e.Params[1]) {
		return
	}
	target, key := e.Params[1], girc.ToRFC1459(e.Params[1])
	c.mu.Lock()
	at, asked := c.creating[key]
	delete(c.creating, key)
	if c.explicitCreating[c.foldUnlocked(target)] {
		asked = false
	}
	c.mu.Unlock()
	if !asked || time.Since(at) > creatingWindow {
		return
	}
	channel := client.LookupChannel(target)
	nick := client.GetNick()
	if channel == nil || len(channel.UserList) != 1 || girc.ToRFC1459(channel.UserList[0]) != girc.ToRFC1459(nick) {
		return
	}
	if user := client.LookupUser(nick); user == nil {
		return
	} else if perms, ok := user.Perms.Lookup(target); !ok || !perms.IsAdmin() {
		return
	}
	w := c.owner
	w.mu.Lock()
	conversation := c.conversation(target).ID
	w.mu.Unlock()
	if !c.registersCreatedChannels() {
		return
	}
	m := c.manager
	m.mu.Lock()
	closed := m.closed
	if !closed {
		m.wg.Add(1)
	}
	m.mu.Unlock()
	if closed {
		return
	}
	go func() {
		defer m.wg.Done()
		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
		defer cancel()
		m.autoRegister(ctx, w.ownerID, conversation)
	}()
}

// registersCreatedChannels reads the account's "Register channels I create" preference,
// which is on unless it was turned off.
func (c *connection) registersCreatedChannels() bool {
	store := c.owner.store
	if store == nil {
		return true
	}
	raw, err := store.AppData(c.owner.ownerID, "client-preferences.updated")
	if err != nil || len(raw) == 0 {
		return err == nil
	}
	var doc struct {
		Preferences struct {
			RegisterChannels *bool `json:"registerChannels"`
		} `json:"preferences"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.Preferences.RegisterChannels == nil {
		return true
	}
	return *doc.Preferences.RegisterChannels
}

func (m *Manager) autoRegister(ctx context.Context, owner, conversation string) {
	_, err := m.RegisterChannel(ctx, owner, conversation)
	var failure *ServiceError
	switch {
	case err == nil:
		w := m.space(owner)
		if w.store != nil {
			if err := w.store.SetAutoRegistered(owner, conversation, true); err != nil {
				slog.Error("Recording an automatic channel registration failed", "error", err)
			}
		}
		m.registrationEvent(owner, conversation, "registered", "")
	case errors.As(err, &failure) && failure.Status == 409 && strings.Contains(failure.Detail, "already registered"):
		// Someone else's channel that happened to be empty: nothing to claim.
	case errors.As(err, &failure):
		m.registrationEvent(owner, conversation, "registration-failed", failure.Detail)
	default:
		m.registrationEvent(owner, conversation, "registration-failed", "ChanServ couldn’t be reached. Register the channel from its settings.")
	}
}

// registrationEvent adds a timeline event about the automatic registration to the channel.
func (m *Manager) registrationEvent(owner, conversation, kind, text string) {
	w := m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.conversations[conversation]; !ok {
		return
	}
	data := &model.MembershipEvent{Type: kind, ActorID: w.self.ID, Text: text}
	msg := Message{ID: id(), ConversationID: conversation, SenderID: w.self.ID, Text: text, CreatedAt: time.Now().UTC(), Kind: "event", Event: data, Reactions: []Reaction{}}
	w.messages[conversation] = append(w.messages[conversation], msg)
	if len(w.messages[conversation]) > 500 {
		w.messages[conversation] = append([]Message(nil), w.messages[conversation][len(w.messages[conversation])-500:]...)
	}
	w.boundCache(conversation)
	w.emit("message.created", msg)
}
