package irc

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A message you sent is stored under the local ID of its send, while other readers store it
// under the shared ID that a message link carries. Both must open it.
func TestMessageLinksFindYourOwnMessageByItsSharedID(t *testing.T) {
	f := newSavedFixture(t)
	w := f.m.space("alice")
	w.mu.Lock()
	w.emit("message.created", Message{ID: "local-send", UpstreamID: "sent1", ConversationID: f.dev.ID, SenderID: "you", Kind: "message", Text: "my own words", CreatedAt: time.Unix(3000, 0).UTC(), Reactions: []Reaction{}})
	w.mu.Unlock()
	for i := range 6 {
		f.post(t, f.dev, "one:later"+string(rune('a'+i)), "later", time.Unix(int64(3001+i), 0).UTC())
	}

	page, err := f.m.HistoryAround("alice", f.dev.ID, "one:sent1", 3)
	if err != nil || len(page.Items) != 3 || page.Items[1].ID != "local-send" {
		t.Fatalf("around shared ID: %+v %v", page.Items, err)
	}
	if page, err = f.m.HistoryAround("alice", f.dev.ID, "local-send", 1); err != nil || page.Items[0].ID != "local-send" {
		t.Fatalf("around local ID: %+v %v", page.Items, err)
	}
	if _, err = f.m.HistoryAround("alice", f.dev.ID, "two:sent1", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another network's shared ID = %v, want ErrNotFound", err)
	}

	ctx := context.Background()
	for id, want := range map[string]struct{ message, conversation string }{
		"one:sent1":  {"local-send", f.dev.ID},
		"local-send": {"local-send", f.dev.ID},
		"one:abc":    {"one:abc", f.dev.ID},
		"two:abc":    {"two:abc", f.other.ID},
	} {
		msg, err := f.m.LocateMessage(ctx, "alice", id)
		if err != nil || msg.ID != want.message || msg.ConversationID != want.conversation {
			t.Fatalf("LocateMessage(%q) = %q in %q, %v; want %q in %q", id, msg.ID, msg.ConversationID, err, want.message, want.conversation)
		}
	}
	for _, id := range []string{"one:missing", "one:", "three:abc", "sent1"} {
		if _, err := f.m.LocateMessage(ctx, "alice", id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("LocateMessage(%q) = %v, want ErrNotFound", id, err)
		}
	}
	if _, err := f.m.LocateMessage(ctx, "bob", "one:abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another account = %v, want ErrNotFound", err)
	}
	// A conversation you closed no longer opens its messages.
	w.mu.Lock()
	delete(w.conversations, f.other.ID)
	w.mu.Unlock()
	if _, err := f.m.LocateMessage(ctx, "alice", "two:abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("closed conversation = %v, want ErrNotFound", err)
	}
}
