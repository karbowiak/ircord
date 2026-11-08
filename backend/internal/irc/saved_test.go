package irc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type savedFixture struct {
	m     *Manager
	store *storage.Store
	dir   string
	dev   Conversation // #dev on network "one"
	other Conversation // #dev on network "two": same name, different network
}

// newSavedFixture gives alice two networks that both have a #dev channel, each
// with a message carrying the same upstream msgid, and an empty account bob.
func newSavedFixture(t *testing.T) savedFixture {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(owner, owner); err != nil {
			t.Fatal(err)
		}
	}
	m := New(Options{Store: store})
	t.Cleanup(func() { m.Close(); store.Close() })
	f := savedFixture{m: m, store: store, dir: dir}
	w := m.space("alice")
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, network := range []string{"one", "two"} {
		c := &connection{owner: w, manager: m, config: NetworkConfig{ID: network}}
		conv := c.conversation("#dev")
		conv.Joined = true
		w.conversations[conv.ID] = conv
		w.emit("message.created", Message{ID: network + ":abc", UpstreamID: "abc", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "notes from " + network, CreatedAt: time.Unix(int64(1000+i), 0).UTC(), Reactions: []Reaction{}})
		if i == 0 {
			f.dev = conv
		} else {
			f.other = conv
		}
	}
	if w.storageErr != nil {
		t.Fatal(w.storageErr)
	}
	return f
}

func (f savedFixture) post(t *testing.T, conv Conversation, id, text string, at time.Time) {
	t.Helper()
	w := f.m.space("alice")
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emit("message.created", Message{ID: id, UpstreamID: strings.TrimPrefix(id, conv.NetworkID+":"), ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: text, CreatedAt: at, Reactions: []Reaction{}})
	if w.storageErr != nil {
		t.Fatal(w.storageErr)
	}
}

func savedEvents(t *testing.T, m *Manager, owner string) []string {
	t.Helper()
	events, _, ok, stop := m.Subscribe(owner, 0)
	defer stop()
	if !ok {
		t.Fatal("replay unavailable")
	}
	kinds := []string{}
	for _, e := range events {
		if strings.HasPrefix(e.Type, "saved.") {
			kinds = append(kinds, e.Type)
		}
	}
	return kinds
}

func TestSaveMessageIsIdempotentAndPrivate(t *testing.T) {
	f := newSavedFixture(t)
	first, err := f.m.SaveMessage("alice", f.dev.ID, "one:abc")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "available" || first.Message == nil || first.Message.Text != "notes from one" || first.NetworkID != "one" {
		t.Fatalf("saved item: %+v", first)
	}
	// A repeated or retried save returns the same bookmark and changes nothing.
	again, err := f.m.SaveMessage("alice", f.dev.ID, "one:abc")
	if err != nil || !again.SavedAt.Equal(first.SavedAt) {
		t.Fatalf("second save: %+v %v", again, err)
	}
	// Bookmarks are keyed by the canonical ID only: an upstream msgid is not an
	// alias that PUT accepts but DELETE would ignore.
	if _, err = f.m.SaveMessage("alice", f.dev.ID, "abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("saved by upstream msgid: %v", err)
	}
	refs, err := f.m.SavedRefs("alice")
	if err != nil || len(refs) != 1 || refs[0].MessageID != "one:abc" {
		t.Fatalf("refs: %+v %v", refs, err)
	}
	if got := savedEvents(t, f.m, "alice"); len(got) != 1 || got[0] != "saved.added" {
		t.Fatalf("events: %v", got)
	}

	// Another account can neither see, save nor remove alice's bookmark.
	if _, err = f.m.SaveMessage("bob", f.dev.ID, "one:abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob saved alice's message: %v", err)
	}
	if err = f.m.UnsaveMessage("bob", "one:abc"); err != nil {
		t.Fatal(err)
	}
	page, err := f.m.SavedMessages("bob", "", "", 30)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("bob's list: %+v %v", page, err)
	}
	if got := savedEvents(t, f.m, "bob"); len(got) != 0 {
		t.Fatalf("bob received saved events: %v", got)
	}
	if refs, _ = f.m.SavedRefs("alice"); len(refs) != 1 {
		t.Fatal("bob removed alice's bookmark")
	}

	// Removing is idempotent too, and only removes the reference.
	for range 2 {
		if err = f.m.UnsaveMessage("alice", "one:abc"); err != nil {
			t.Fatal(err)
		}
	}
	if got := savedEvents(t, f.m, "alice"); len(got) != 2 || got[1] != "saved.removed" {
		t.Fatalf("events after removal: %v", got)
	}
	if msg, err := f.store.Message("alice", f.dev.ID, "one:abc"); err != nil || msg.Text != "notes from one" {
		t.Fatalf("message changed: %+v %v", msg, err)
	}
}

func TestSavedMessagesNeverResolveAcrossSameNamedChannels(t *testing.T) {
	f := newSavedFixture(t)
	if f.dev.ID == f.other.ID || f.dev.Name != f.other.Name {
		t.Fatal("fixture needs two distinct #dev conversations")
	}
	// The message of network one is not a message of network two's #dev.
	if _, err := f.m.SaveMessage("alice", f.other.ID, "one:abc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("saved through the wrong conversation: %v", err)
	}
	if _, err := f.m.SaveMessage("alice", f.other.ID, "two:abc"); err != nil {
		t.Fatal(err)
	}
	page, err := f.m.SavedMessages("alice", "", "", 30)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("list: %+v %v", page, err)
	}
	item := page.Items[0]
	if item.ConversationID != f.other.ID || item.NetworkID != "two" || item.Message.Text != "notes from two" || item.Conversation.ID != f.other.ID {
		t.Fatalf("resolved to the wrong content: %+v", item)
	}
}

func TestSavedMessagesFollowDeletionAndAccess(t *testing.T) {
	f := newSavedFixture(t)
	for _, id := range []string{"one:abc", "two:abc"} {
		conv := f.dev
		if strings.HasPrefix(id, "two") {
			conv = f.other
		}
		if _, err := f.m.SaveMessage("alice", conv.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	w := f.m.space("alice")
	w.mu.Lock()
	// The author deletes the first message: the stored copy becomes a stub.
	w.emit("message.updated", Message{ID: "one:abc", UpstreamID: "abc", ConversationID: f.dev.ID, SenderID: "peer", Kind: "message", Text: "Message deleted", Deleted: true, CreatedAt: time.Unix(1000, 0).UTC(), Reactions: []Reaction{}})
	// Alice leaves network two's #dev.
	delete(w.conversations, f.other.ID)
	w.mu.Unlock()

	page, err := f.m.SavedMessages("alice", "", "", 30)
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("list: %+v %v", page, err)
	}
	byID := map[string]SavedItem{}
	for _, item := range page.Items {
		byID[item.MessageID] = item
	}
	deleted := byID["one:abc"]
	if deleted.Status != "deleted" || deleted.Message == nil || deleted.Message.Text != "" || deleted.Message.SenderID != "peer" || deleted.Conversation == nil {
		t.Fatalf("deleted stub: %+v", deleted)
	}
	gone := byID["two:abc"]
	if gone.Status != "unavailable" || gone.Message != nil || gone.Conversation != nil {
		t.Fatalf("left conversation leaked content: %+v", gone)
	}
	// Searching never finds deleted or inaccessible text.
	for _, q := range []string{"notes", "deleted"} {
		if page, err = f.m.SavedMessages("alice", "", q, 30); err != nil || len(page.Items) != 0 {
			t.Fatalf("search %q: %+v %v", q, page, err)
		}
	}
	// A deleted message cannot be newly saved, and removal leaves other bookmarks alone.
	if err = f.m.UnsaveMessage("alice", "one:abc"); err != nil {
		t.Fatal(err)
	}
	if _, err = f.m.SaveMessage("alice", f.dev.ID, "one:abc"); err == nil {
		t.Fatal("saved a deleted message")
	}
	if refs, _ := f.m.SavedRefs("alice"); len(refs) != 1 || refs[0].MessageID != "two:abc" {
		t.Fatalf("refs: %+v", refs)
	}
}

func TestSavedMessagesPageAndSearch(t *testing.T) {
	f := newSavedFixture(t)
	for i := range 5 {
		id := "one:m" + string(rune('a'+i))
		text := "plain"
		if i%2 == 0 {
			text = "Deploy note"
		}
		f.post(t, f.dev, id, text, time.Unix(int64(2000+i), 0).UTC())
		if _, err := f.m.SaveMessage("alice", f.dev.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	seen := []string{}
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 3 {
			t.Fatal("pagination did not end")
		}
		page, err := f.m.SavedMessages("alice", cursor, "", 2)
		if err != nil || len(page.Items) > 2 {
			t.Fatalf("page: %+v %v", page, err)
		}
		for _, item := range page.Items {
			seen = append(seen, item.MessageID)
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if strings.Join(seen, ",") != "one:me,one:md,one:mc,one:mb,one:ma" {
		t.Fatalf("order: %v", seen)
	}
	page, err := f.m.SavedMessages("alice", "", "deploy", 30)
	if err != nil || len(page.Items) != 3 {
		t.Fatalf("search: %+v %v", page, err)
	}
	if _, err = f.m.SavedMessages("alice", "not-a-cursor", "", 30); err == nil {
		t.Fatal("accepted a forged cursor")
	}
	if _, err = f.m.SavedMessages("alice", "", "", 101); err == nil {
		t.Fatal("accepted an unbounded page")
	}
}

func TestSavedMessagesAreBounded(t *testing.T) {
	f := newSavedFixture(t)
	previous := SavedLimit
	SavedLimit = 1
	t.Cleanup(func() { SavedLimit = previous })
	if _, err := f.m.SaveMessage("alice", f.dev.ID, "one:abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.SaveMessage("alice", f.other.ID, "two:abc"); !errors.Is(err, ErrSavedLimit) {
		t.Fatalf("limit: %v", err)
	}
	// Saving an already saved message still succeeds at the limit.
	if _, err := f.m.SaveMessage("alice", f.dev.ID, "one:abc"); err != nil {
		t.Fatal(err)
	}
}

func TestSavedMessagesSurviveRestartAndGoWithTheirNetwork(t *testing.T) {
	f := newSavedFixture(t)
	for _, v := range []struct {
		conv Conversation
		id   string
	}{{f.dev, "one:abc"}, {f.other, "two:abc"}} {
		if _, err := f.m.SaveMessage("alice", v.conv.ID, v.id); err != nil {
			t.Fatal(err)
		}
	}
	f.m.Close()
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	refs, err := store.SavedRefs("alice")
	if err != nil || len(refs) != 2 {
		t.Fatalf("after restart: %+v %v", refs, err)
	}
	// Removing a network removes its history, and with it the bookmarks into it,
	// also into a conversation that was closed (its entity removed) before.
	if _, err = store.Commit("alice", "conversation.removed", model.Removal{ID: f.dev.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Commit("alice", "network.removed", model.Removal{ID: "one"}); err != nil {
		t.Fatal(err)
	}
	if refs, err = store.SavedRefs("alice"); err != nil || len(refs) != 1 || refs[0].NetworkID != "two" {
		t.Fatalf("after network removal: %+v %v", refs, err)
	}
}

func TestSavedMessagesWithoutStorage(t *testing.T) {
	m := New(Options{})
	t.Cleanup(m.Close)
	w := m.space("alice")
	w.mu.Lock()
	c := &connection{owner: w, manager: m, config: NetworkConfig{ID: "one"}}
	conv := c.conversation("#dev")
	w.conversations[conv.ID] = conv
	w.messages[conv.ID] = []Message{{ID: "one:abc", UpstreamID: "abc", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "hello", Reactions: []Reaction{}}}
	w.mu.Unlock()
	for range 2 {
		if _, err := m.SaveMessage("alice", conv.ID, "one:abc"); err != nil {
			t.Fatal(err)
		}
	}
	page, err := m.SavedMessages("alice", "", "hel", 30)
	if err != nil || len(page.Items) != 1 || page.Items[0].Message.Text != "hello" {
		t.Fatalf("list: %+v %v", page, err)
	}
	if err = m.UnsaveMessage("alice", "one:abc"); err != nil {
		t.Fatal(err)
	}
	if refs, _ := m.SavedRefs("alice"); len(refs) != 0 {
		t.Fatalf("refs: %+v", refs)
	}
	// Deleting the network drops its bookmarks here too, also after the conversation closed.
	if _, err = m.SaveMessage("alice", conv.ID, "one:abc"); err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.networks["one"] = Network{ID: "one"}
	delete(w.conversations, conv.ID)
	w.mu.Unlock()
	if err = m.DeleteNetwork("alice", "one"); err != nil {
		t.Fatal(err)
	}
	if refs, _ := m.SavedRefs("alice"); len(refs) != 0 {
		t.Fatalf("bookmarks outlived their network: %+v", refs)
	}
}

func TestSavedSearchFillsPagesPastInaccessibleMatches(t *testing.T) {
	f := newSavedFixture(t)
	for i := range 4 {
		id := "two:n" + string(rune('a'+i))
		f.post(t, f.other, id, "match", time.Unix(int64(3000+i), 0).UTC())
		if _, err := f.m.SaveMessage("alice", f.other.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	f.post(t, f.dev, "one:keep", "match", time.Unix(2000, 0).UTC())
	if _, err := f.m.SaveMessage("alice", f.dev.ID, "one:keep"); err != nil {
		t.Fatal(err)
	}
	w := f.m.space("alice")
	w.mu.Lock()
	delete(w.conversations, f.other.ID) // the four newest matches become inaccessible
	w.mu.Unlock()
	page, err := f.m.SavedMessages("alice", "", "match", 2)
	if err != nil || len(page.Items) != 1 || page.Items[0].MessageID != "one:keep" || page.NextCursor != nil {
		t.Fatalf("search did not read past inaccessible matches: %+v %v", page, err)
	}
}

func TestHistoryAroundReturnsAWindowCentredOnTheMessage(t *testing.T) {
	f := newSavedFixture(t)
	for i := range 20 {
		f.post(t, f.dev, fmt.Sprintf("one:h%02d", i), fmt.Sprintf("line %d", i), time.Unix(int64(5000+i), 0).UTC())
	}
	page, err := f.m.HistoryAround("alice", f.dev.ID, "one:h05", 5)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, m := range page.Items {
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, ",") != "one:h03,one:h04,one:h05,one:h06,one:h07" || !page.HasNewer || page.NextCursor == nil {
		t.Fatalf("window: %v hasNewer=%v next=%v", ids, page.HasNewer, page.NextCursor)
	}
	// The cursor continues to older history through the ordinary read.
	older, err := f.m.HistoryContext(context.Background(), "alice", f.dev.ID, *page.NextCursor, 10)
	if err != nil || len(older.Items) == 0 || older.Items[len(older.Items)-1].ID != "one:h02" {
		t.Fatalf("older: %+v %v", older, err)
	}
	// At the live end there is nothing newer.
	if page, err = f.m.HistoryAround("alice", f.dev.ID, "one:h19", 5); err != nil || page.HasNewer || page.Items[len(page.Items)-1].ID != "one:h19" {
		t.Fatalf("tail window: %+v %v", page, err)
	}
	// Another conversation's message, or another account, is not found.
	if _, err = f.m.HistoryAround("alice", f.other.ID, "one:h05", 5); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-conversation: %v", err)
	}
	if _, err = f.m.HistoryAround("bob", f.dev.ID, "one:h05", 5); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-account: %v", err)
	}
}
