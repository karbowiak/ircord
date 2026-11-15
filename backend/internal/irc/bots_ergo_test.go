package irc

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type botErgoFixture struct {
	m             *Manager
	home          NetworkConfig
	room          string
	conversations map[string]Conversation
	roboID        string
	// join connects another account and joins it to the room.
	join func(account string)
}

// botErgo starts a disposable Ergo whose moderation operator class has the
// given capabilities, with an owner and a bot account joined to one channel.
// The account "late" is registered but joins only when a test asks.
func botErgo(t *testing.T, ctx context.Context, capabilities ...string) botErgoFixture {
	t.Helper()
	operPassword := id()
	hash, err := bcrypt.GenerateFromPassword([]byte(operPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	host, port, associated := isolatedErgo(t, func(cfg map[string]any) {
		cfg["oper-classes"] = map[string]any{"ircord-moderation": map[string]any{"title": "IRCord moderation", "capabilities": capabilities}}
		cfg["opers"] = map[string]any{"ircord-moderation": map[string]any{"class": "ircord-moderation", "hidden": true, "password": string(hash)}}
	})
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	home := NetworkConfig{ID: "home", Host: host, Port: port, Name: "Fixture"}
	password := id() + id()
	for _, account := range []string{"owner", "robo", "late"} {
		if _, err = store.CreateAccount(account, account); err != nil {
			t.Fatal(err)
		}
		if err = associated.Register(ctx, account, password); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.GrantOwner("owner", storage.OwnerFromOwner, "test"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store, Home: home, AssociatedServer: associated, Moderation: &ModerationLogin{Operator: "ircord-moderation", Password: operPassword}, AllowPrivate: true, ReconnectMin: time.Hour})
	t.Cleanup(m.Close)
	room := "#bots" + id()[:8]
	conversations := map[string]Conversation{}
	connect := func(account string) {
		cfg := home
		cfg.Nick = account
		cfg.Auth = &model.AuthInput{Method: "sasl-plain", Account: account, Password: &password}
		if _, err := m.Connect(account, cfg); err != nil {
			t.Fatal(err)
		}
		if err := m.WaitConnected(ctx, account, home.ID); err != nil {
			t.Fatal(err)
		}
		if conversations[account], err = m.Join(account, home.ID, room); err != nil {
			t.Fatal(err)
		}
		eventually(t, func() bool { return findConversation(t, m, account, home.ID, room).Joined })
	}
	connect("robo")
	connect("owner")
	eventually(t, func() bool { return len(findConversation(t, m, "owner", home.ID, room).MemberIDs) == 2 })
	roboID := ""
	for _, memberID := range findConversation(t, m, "owner", home.ID, room).MemberIDs {
		if v, err := m.Member("owner", memberID); err == nil && v.Nick == "robo" {
			roboID = v.ID
		}
	}
	if roboID == "" {
		t.Fatal("the owner does not see robo")
	}
	return botErgoFixture{m: m, home: home, room: room, conversations: conversations, roboID: roboID, join: connect}
}

// staleRole overwrites what owner's workspace believes about nick, as if it had missed a change.
func staleRole(t *testing.T, m *Manager, owner, network, room, nick, role string) {
	t.Helper()
	w := m.space(owner)
	for _, memberID := range findConversation(t, m, owner, network, room).MemberIDs {
		w.mu.Lock()
		if v := w.members[memberID]; strings.EqualFold(v.Nick, nick) {
			v.Role = role
			w.members[memberID] = v
		}
		w.mu.Unlock()
	}
}

func roleIn(t *testing.T, m *Manager, owner, network, room, nick string) string {
	t.Helper()
	for _, memberID := range findConversation(t, m, owner, network, room).MemberIDs {
		if v, err := m.Member(owner, memberID); err == nil && strings.EqualFold(v.Nick, nick) {
			return v.Role
		}
	}
	return ""
}

// Ergo v2.19.1 allows SAMODE only to operator classes with the samode
// capability; history and nofakelag alone are refused.
func TestErgoBotMarkNeedsTheSamodeCapability(t *testing.T) {
	if os.Getenv("IRCORD_MODERATION_INTEGRATION") != "1" {
		t.Skip("set IRCORD_MODERATION_INTEGRATION=1 for an isolated Ergo with a moderation operator")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	f := botErgo(t, ctx, "history", "nofakelag")
	if _, err := f.m.SetBot(ctx, "owner", f.home.ID, f.roboID, true); !errors.Is(err, ErrBotRefused) {
		t.Fatalf("SAMODE without the samode capability: %v", err)
	}
}

// A server owner marks a bot through the moderation login; the bot's own
// messages then carry the bot tag, and an account that joins later learns it
// from WHOX flags. Removing the mark reaches both again.
func TestErgoServerOwnerMarksABot(t *testing.T) {
	if os.Getenv("IRCORD_MODERATION_INTEGRATION") != "1" {
		t.Skip("set IRCORD_MODERATION_INTEGRATION=1 for an isolated Ergo with a moderation operator")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	f := botErgo(t, ctx, "history", "nofakelag", "samode")
	m, home, room, conversations, roboID := f.m, f.home, f.room, f.conversations, f.roboID
	mark, err := m.SetBot(ctx, "owner", home.ID, roboID, true)
	if err != nil {
		t.Fatal(err)
	}
	if mark.Member.Role != "bot" {
		t.Fatalf("mark: %+v", mark)
	}
	f.join("late")
	eventually(t, func() bool { return roleIn(t, m, "late", home.ID, room, "robo") == "bot" })
	if roleIn(t, m, "late", home.ID, room, "owner") == "bot" {
		t.Fatal("WHOX flags marked a person who is not a bot")
	}
	// WHOIS answers RPL_WHOISBOT for a bot and restores a missed mark.
	staleRole(t, m, "late", home.ID, room, "robo", "member")
	if _, err = m.Whois(ctx, "late", home.ID, "robo"); err != nil {
		t.Fatal(err)
	}
	if got := roleIn(t, m, "late", home.ID, room, "robo"); got != "bot" {
		t.Fatalf("WHOIS of a bot left: %q", got)
	}
	// Ergo set +B on robo's own client: its messages now carry the bot tag.
	if err = m.Send("robo", SendInput{ClientID: "beep", ConversationID: conversations["robo"].ID, Text: "beep"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "late", findConversation(t, m, "late", home.ID, room).ID)) >= 1 })
	if got := roleIn(t, m, "late", home.ID, room, "robo"); got != "bot" {
		t.Fatalf("after a tagged message: %q", got)
	}
	// Ergo tags only the opening BATCH line of a multiline message.
	if err = m.Send("robo", SendInput{ClientID: "lines", ConversationID: conversations["robo"].ID, Text: "line one\nline two"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "late", findConversation(t, m, "late", home.ID, room).ID)) >= 2 })
	if got := roleIn(t, m, "late", home.ID, room, "robo"); got != "bot" {
		t.Fatalf("after a multiline message: %q", got)
	}
	if _, err = m.SetBot(ctx, "owner", home.ID, roboID, false); err != nil {
		t.Fatal(err)
	}
	if roleIn(t, m, "owner", home.ID, room, "robo") != "member" {
		t.Fatal("the owner still sees the mark")
	}
	eventually(t, func() bool { return roleIn(t, m, "late", home.ID, room, "robo") == "member" })
	// A complete WHOIS without RPL_WHOISBOT clears a stale mark.
	staleRole(t, m, "late", home.ID, room, "robo", "bot")
	if _, err = m.Whois(ctx, "late", home.ID, "robo"); err != nil {
		t.Fatal(err)
	}
	if got := roleIn(t, m, "late", home.ID, room, "robo"); got != "member" {
		t.Fatalf("WHOIS of someone who is no longer a bot left: %q", got)
	}
	// The next live message without the tag keeps it cleared.
	if err = m.Send("robo", SendInput{ClientID: "plain", ConversationID: conversations["robo"].ID, Text: "plain"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return len(page(m, "late", findConversation(t, m, "late", home.ID, room).ID)) >= 3 })
	if got := roleIn(t, m, "late", home.ID, room, "robo"); got != "member" {
		t.Fatalf("after an untagged message: %q", got)
	}
}
