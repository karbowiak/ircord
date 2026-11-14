package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ircord/backend/internal/auth"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/storage"
)

func TestServerOwnerDeletionAuditKeepsNoMessageText(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	s := &Server{options: Options{Store: store}}
	ctx := context.WithValue(context.Background(), principalKey{}, auth.Principal{Account: storage.Account{ID: "a1", Username: "owner"}})
	r := irc.RedactResult{Authority: irc.AuthorityOwner, Channel: "#general", Sender: "spammer", Original: irc.Message{UpstreamID: "msg-1", SenderAccount: "spamacct", Text: "the abusive text"}}
	s.auditDeletion(ctx, r, "spam", nil, 2)
	s.auditDeletion(ctx, r, "", errors.Join(irc.ErrRedactRefused, errors.New("This message does not exist or is too old")), 0)
	events, err := store.OwnerEvents(ctx, 10)
	if err != nil || len(events) != 2 {
		t.Fatal(events, err)
	}
	deleted, refused := events[1], events[0]
	if deleted.Actor != "owner" || deleted.Action != "message-delete" || deleted.Target != "#general · spammer (spamacct) · msg-1" || deleted.Reason != "spam" || deleted.Outcome != "deleted with 2 files" {
		t.Fatalf("deleted: %+v", deleted)
	}
	if !strings.HasPrefix(refused.Outcome, "refused") {
		t.Fatalf("refused: %+v", refused)
	}
	for _, e := range events {
		if strings.Contains(e.Target+e.Reason+e.Outcome, "abusive") {
			t.Fatalf("the audit log kept message text: %+v", e)
		}
	}
}
