package irc

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

// A server owner who is not a channel operator deletes another person's message
// on a disposable Ergo with an operator class limited to history. The moderation
// login is not in the channel, so it gets only the labeled ACK, and the author's
// account applies the REDACT Ergo relays from it. Then a channel operator deletes
// one member's recent messages with their own connection.
func TestErgoModeratorsDeleteOthersMessages(t *testing.T) {
	if os.Getenv("IRCORD_MODERATION_INTEGRATION") != "1" {
		t.Skip("set IRCORD_MODERATION_INTEGRATION=1 for an isolated Ergo with a moderation operator")
	}
	operPassword := id()
	hash, err := bcrypt.GenerateFromPassword([]byte(operPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	host, port, associated := isolatedErgo(t, func(cfg map[string]any) {
		cfg["oper-classes"] = map[string]any{"ircord-moderation": map[string]any{"title": "IRCord moderation", "capabilities": []string{"history", "nofakelag"}}}
		cfg["opers"] = map[string]any{"ircord-moderation": map[string]any{"class": "ircord-moderation", "hidden": true, "password": string(hash)}}
	})
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	home := NetworkConfig{ID: "home", Host: host, Port: port, Name: "Fixture"}
	password := id() + id()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for _, account := range []string{"owner", "author", "member"} {
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
	defer m.Close()
	room := "#moderation" + id()[:8]
	conversations := map[string]Conversation{}
	// The author joins first and becomes the channel operator; the owner and the
	// third member hold no channel status.
	for _, account := range []string{"author", "owner", "member"} {
		cfg := home
		cfg.Nick = account
		cfg.Auth = &model.AuthInput{Method: "sasl-plain", Account: account, Password: &password}
		if _, err = m.Connect(account, cfg); err != nil {
			t.Fatal(err)
		}
		if err = m.WaitConnected(ctx, account, home.ID); err != nil {
			t.Fatal(err)
		}
		if conversations[account], err = m.Join(account, home.ID, room); err != nil {
			t.Fatal(err)
		}
		eventually(t, func() bool { return findConversation(t, m, account, home.ID, room).Joined })
	}
	eventually(t, func() bool { return len(findConversation(t, m, "owner", home.ID, room).MemberIDs) == 3 })
	if err = m.Send("author", SendInput{ClientID: "abuse", ConversationID: conversations["author"].ID, Text: "abusive text"}); err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"author", "owner", "member"} {
		eventually(t, func() bool { return len(page(m, account, conversations[account].ID)) == 1 })
	}
	target := page(m, "owner", conversations["owner"].ID)[0]
	// The member is neither the author, an operator nor an owner.
	if _, err = m.RedactAs(ctx, "member", conversations["member"].ID, page(m, "member", conversations["member"].ID)[0].ID, ""); !errors.Is(err, ErrDeleteForbidden) {
		t.Fatalf("an ordinary member deleted someone else's message: %v", err)
	}
	r, err := m.RedactAs(ctx, "owner", conversations["owner"].ID, target.ID, "test moderation")
	if err != nil {
		t.Fatal(err)
	}
	if r.Authority != AuthorityOwner || !r.Message.Deleted {
		t.Fatalf("result %+v", r)
	}
	for _, account := range []string{"author", "owner", "member"} {
		eventually(t, func() bool { return page(m, account, conversations[account].ID)[0].Deleted })
	}
	// Repeating it is refused locally and never reaches the network.
	if _, err = m.RedactAs(ctx, "owner", conversations["owner"].ID, target.ID, ""); !errors.Is(err, ErrAlreadyDeleted) {
		t.Fatalf("second deletion: %v", err)
	}

	// The channel operator deletes the member's recent messages with their own
	// connection, one confirmed REDACT at a time.
	for i := range 3 {
		if err = m.Send("member", SendInput{ClientID: "spam" + id()[:6], ConversationID: conversations["member"].ID, Text: "spam " + string(rune('a'+i))}); err != nil {
			t.Fatal(err)
		}
	}
	for _, account := range []string{"author", "owner", "member"} {
		eventually(t, func() bool { return len(page(m, account, conversations[account].ID)) == 4 })
	}
	memberID := page(m, "author", conversations["author"].ID)[3].SenderID
	finished := make(chan PurgeJob, 1)
	job, err := m.StartPurge(ctx, "author", conversations["author"].ID, memberID, time.Hour, "", func(j PurgeJob) { finished <- j })
	if err != nil {
		t.Fatal(err)
	}
	if job.Authority != AuthorityOperator || job.Total != 3 {
		t.Fatalf("job %+v", job)
	}
	select {
	case done := <-finished:
		if done.Deleted != 3 || done.Failed != 0 || done.TooOld != 0 {
			t.Fatalf("finished %+v", done)
		}
	case <-ctx.Done():
		t.Fatal("the deletion job did not finish")
	}
	for _, account := range []string{"author", "owner", "member"} {
		eventually(t, func() bool {
			for _, msg := range page(m, account, conversations[account].ID) {
				if !msg.Deleted {
					return false
				}
			}
			return true
		})
	}
}
