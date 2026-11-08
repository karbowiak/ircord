package irc

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

// linkFixture is the saved-message fixture with its channels stored as
// conversations, as the live workspace keeps them.
func newLinkFixture(t *testing.T) savedFixture {
	t.Helper()
	f := newSavedFixture(t)
	w := f.m.space("alice")
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, conv := range []Conversation{f.dev, f.other} {
		w.emit("conversation.updated", conv)
	}
	if w.storageErr != nil {
		t.Fatal(w.storageErr)
	}
	return f
}

func (f savedFixture) postAs(t *testing.T, conv Conversation, id, sender, text string, at time.Time, edit func(*Message)) {
	t.Helper()
	w := f.m.space("alice")
	w.mu.Lock()
	defer w.mu.Unlock()
	msg := Message{ID: id, UpstreamID: id, ConversationID: conv.ID, SenderID: sender, Kind: "message", Text: text, CreatedAt: at, Reactions: []Reaction{}}
	if edit != nil {
		edit(&msg)
	}
	w.emit("message.created", msg)
	if w.storageErr != nil {
		t.Fatal(w.storageErr)
	}
}

func (f savedFixture) links(t *testing.T, owner string, q LinkQuery) LinkPage {
	t.Helper()
	if q.Limit == 0 {
		q.Limit = 30
	}
	page, err := f.m.Links(owner, q)
	if err != nil {
		t.Fatal(err)
	}
	return page
}

type cardSummary struct {
	url     string
	count   int
	sources []string
}

func summarize(page LinkPage) []cardSummary {
	out := []cardSummary{}
	for _, c := range page.Items {
		s := cardSummary{url: c.URL, count: c.Count}
		for _, src := range c.Sources {
			s.sources = append(s.sources, src.MessageID)
		}
		out = append(out, s)
	}
	return out
}

func equalCards(a, b []cardSummary) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].url != b[i].url || a[i].count != b[i].count || len(a[i].sources) != len(b[i].sources) {
			return false
		}
		for j := range a[i].sources {
			if a[i].sources[j] != b[i].sources[j] {
				return false
			}
		}
	}
	return true
}

func TestLinkCatalogGroupsRepeatsAndFiltersBySource(t *testing.T) {
	f := newLinkFixture(t)
	at := func(s int64) time.Time { return time.Unix(2000+s, 0).UTC() }
	f.postAs(t, f.dev, "one:m1", "peer", "look https://a.test/x", at(1), nil)
	f.postAs(t, f.other, "two:m2", "carol", "again https://a.test/x#intro and https://b.test/?id=1", at(2), nil)
	f.postAs(t, f.dev, "one:m3", "peer", "```\nhttps://a.test/x\n```", at(3), nil)
	f.postAs(t, f.dev, "one:m4", "peer", "different article https://b.test/?id=2", at(1), nil)

	got := summarize(f.links(t, "alice", LinkQuery{}))
	want := []cardSummary{
		{"https://b.test/?id=1", 1, []string{"two:m2"}},
		{"https://a.test/x#intro", 2, []string{"two:m2", "one:m1"}},
		{"https://b.test/?id=2", 1, []string{"one:m4"}},
	}
	if !equalCards(got, want) {
		t.Fatalf("catalog:\n got %+v\nwant %+v", got, want)
	}
	page := f.links(t, "alice", LinkQuery{})
	src := page.Items[1].Sources[0]
	if src.ConversationID != f.other.ID || src.NetworkID != "two" || src.SenderID != "carol" || src.Excerpt != "again https://a.test/x#intro and https://b.test/?id=1" || !src.PostedAt.Equal(at(2)) {
		t.Fatalf("source context: %+v", src)
	}

	// Every filter narrows the counts and sources too, never just the cards.
	if got = summarize(f.links(t, "alice", LinkQuery{Conversations: []string{f.dev.ID}})); !equalCards(got, []cardSummary{{"https://b.test/?id=2", 1, []string{"one:m4"}}, {"https://a.test/x", 1, []string{"one:m1"}}}) {
		t.Fatalf("channel view: %+v", got)
	}
	if got = summarize(f.links(t, "alice", LinkQuery{Network: "two", Host: "A.test"})); !equalCards(got, []cardSummary{{"https://a.test/x#intro", 1, []string{"two:m2"}}}) {
		t.Fatalf("network and host: %+v", got)
	}
	if got = summarize(f.links(t, "alice", LinkQuery{Query: "ARTICLE"})); !equalCards(got, []cardSummary{{"https://b.test/?id=2", 1, []string{"one:m4"}}}) {
		t.Fatalf("text: %+v", got)
	}
	if got = summarize(f.links(t, "alice", LinkQuery{Since: at(2)})); len(got) != 2 || got[1].count != 1 {
		t.Fatalf("since: %+v", got)
	}
	if got = summarize(f.links(t, "alice", LinkQuery{Kinds: []string{"image", "video"}})); len(got) != 0 {
		t.Fatalf("kind: %+v", got)
	}

	// Pages continue after the last card without repeating or skipping one.
	first := f.links(t, "alice", LinkQuery{Limit: 2})
	if len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("first page: %+v", first)
	}
	second := f.links(t, "alice", LinkQuery{Limit: 2, Cursor: *first.NextCursor})
	if len(second.Items) != 1 || second.Items[0].URL != "https://b.test/?id=2" || second.NextCursor != nil {
		t.Fatalf("second page: %+v", summarize(second))
	}
	if _, err := f.m.Links("alice", LinkQuery{Limit: 2, Cursor: "bogus"}); err == nil {
		t.Fatal("accepted a forged cursor")
	}
}

func TestLinkCatalogFollowsEditsDeletionsAndAccess(t *testing.T) {
	f := newLinkFixture(t)
	at := func(s int64) time.Time { return time.Unix(3000+s, 0).UTC() }
	f.postAs(t, f.dev, "one:m1", "peer", "https://a.test/x", at(1), nil)
	f.postAs(t, f.other, "two:m2", "carol", "https://a.test/x secret context", at(2), nil)

	// Deleting one post keeps the other permitted post, and nothing of the deleted one.
	w := f.m.space("alice")
	w.mu.Lock()
	w.emit("message.updated", Message{ID: "two:m2", UpstreamID: "two:m2", ConversationID: f.other.ID, SenderID: "carol", Kind: "message", Text: "Message deleted", Deleted: true, CreatedAt: at(2), Reactions: []Reaction{}})
	w.mu.Unlock()
	page := f.links(t, "alice", LinkQuery{})
	if got := summarize(page); !equalCards(got, []cardSummary{{"https://a.test/x", 1, []string{"one:m1"}}}) {
		t.Fatalf("after deleting one post: %+v", got)
	}
	if len(f.links(t, "alice", LinkQuery{Query: "secret"}).Items) != 0 {
		t.Fatal("a deleted post's text still matches")
	}

	// An edit that drops the link drops it from the catalog.
	f.postAs(t, f.dev, "one:m1", "peer", "nothing to see", at(1), nil)
	if got := f.links(t, "alice", LinkQuery{}); len(got.Items) != 0 {
		t.Fatalf("after edit: %+v", summarize(got))
	}

	// Ignored senders and closed conversations are left out like in the timeline.
	f.postAs(t, f.dev, "one:m5", "dave", "https://c.test/", at(5), nil)
	f.postAs(t, f.other, "two:m6", "erin", "https://d.test/", at(6), nil)
	w.mu.Lock()
	w.emit("member.updated", Member{ID: "dave", Nick: "dave"})
	w.emit("conversation.removed", model.Removal{ID: f.other.ID})
	delete(w.conversations, f.other.ID)
	w.mu.Unlock()
	if _, err := f.m.SaveAppData("alice", "notification-settings.updated", []byte(`{"ignore":["dave"]}`)); err != nil {
		t.Fatal(err)
	}
	if got := f.links(t, "alice", LinkQuery{}); len(got.Items) != 0 {
		t.Fatalf("ignored or closed sources listed: %+v", summarize(got))
	}

	// Another account sees none of it and cannot name alice's channels.
	f.postAs(t, f.dev, "one:m7", "peer", "https://e.test/", at(7), nil)
	if got := f.links(t, "bob", LinkQuery{}); len(got.Items) != 0 {
		t.Fatalf("bob sees alice's links: %+v", summarize(got))
	}
	if _, err := f.m.Links("bob", LinkQuery{Limit: 30, Conversations: []string{f.dev.ID}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob filtered by alice's channel: %v", err)
	}
}

func TestLinkCatalogLeavesOutDirectMessages(t *testing.T) {
	f := newLinkFixture(t)
	dm := Conversation{ID: "one:carol", NetworkID: "one", Kind: "direct", Name: "carol", MemberIDs: []string{}, MemberModes: map[string]string{}}
	w := f.m.space("alice")
	w.mu.Lock()
	w.conversations[dm.ID] = dm
	w.emit("conversation.updated", dm)
	w.mu.Unlock()
	f.postAs(t, dm, "one:d1", "carol", "private https://secret.test/doc", time.Unix(5000, 0).UTC(), nil)
	for _, q := range []LinkQuery{{}, {Network: "one"}, {Query: "secret"}, {Host: "secret.test"}} {
		if got := f.links(t, "alice", q); len(got.Items) != 0 {
			t.Fatalf("%+v listed a direct message link: %+v", q, summarize(got))
		}
	}
	if _, err := f.m.Links("alice", LinkQuery{Limit: 30, Conversations: []string{dm.ID}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("filtering by a direct conversation: %v", err)
	}
}

func TestLinkCatalogCountsOnePostPerMessageAndMatchesSubdomains(t *testing.T) {
	f := newLinkFixture(t)
	at := func(s int64) time.Time { return time.Unix(6000+s, 0).UTC() }
	// One message naming one resource in two spellings is one post, shown by its first spelling.
	f.postAs(t, f.dev, "one:m1", "peer", "https://a.test/x and https://A.test/x#again", at(1), nil)
	f.postAs(t, f.dev, "one:m2", "peer", "https://www.a.test/y", at(2), nil)
	f.postAs(t, f.dev, "one:m3", "peer", "https://evila.test/z", at(3), nil)
	page := f.links(t, "alice", LinkQuery{Host: "a.test"})
	got := summarize(page)
	if !equalCards(got, []cardSummary{{"https://www.a.test/y", 1, []string{"one:m2"}}, {"https://a.test/x", 1, []string{"one:m1"}}}) {
		t.Fatalf("host a.test: %+v", got)
	}
	if src := page.Items[1].Sources; len(src) != 1 || src[0].URL != "https://a.test/x" {
		t.Fatalf("sources of a repeated link: %+v", src)
	}
	// A host filter in Unicode finds posts written in punycode.
	f.postAs(t, f.dev, "one:m4", "peer", "https://xn--bcher-kva.example/", at(4), nil)
	if got := f.links(t, "alice", LinkQuery{Host: "Bücher.example"}); len(got.Items) != 1 {
		t.Fatalf("unicode host filter: %+v", summarize(got))
	}
}

func TestLinkCatalogPagesPastLongIdentities(t *testing.T) {
	f := newLinkFixture(t)
	// Percent-encoding and query separators make identities and cursors long.
	long := "https://ja.wikipedia.org/wiki/" + strings.Repeat("日本", 100)
	// 500 ampersands: a JSON cursor escaped each to six bytes and outgrew its limit.
	amps := "https://a.test/q?" + strings.Repeat("a=1&", 500)
	for i, text := range []string{long, amps, "https://first.test/", "https://second.test/"} {
		f.postAs(t, f.dev, fmt.Sprintf("one:p%d", i), "peer", text, time.Unix(int64(8000-i), 0).UTC(), nil)
	}
	seen := []string{}
	cursor := ""
	for range 10 {
		page, err := f.m.Links("alice", LinkQuery{Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatalf("after %d cards: %v", len(seen), err)
		}
		for _, c := range page.Items {
			seen = append(seen, c.Sources[0].MessageID)
		}
		if page.NextCursor == nil {
			break
		}
		if len(*page.NextCursor) > 4096 {
			t.Fatalf("cursor of %d bytes", len(*page.NextCursor))
		}
		cursor = *page.NextCursor
	}
	if strings.Join(seen, ",") != "one:p0,one:p1,one:p2,one:p3" {
		t.Fatalf("paged: %v", seen)
	}
	// A link whose identity would outgrow the bound stays plain text.
	f.postAs(t, f.dev, "one:p9", "peer", "https://ja.wikipedia.org/wiki/"+strings.Repeat("日本", 300), time.Unix(9900, 0).UTC(), nil)
	if page := f.links(t, "alice", LinkQuery{Limit: 1}); page.Items[0].Sources[0].MessageID == "one:p9" {
		t.Fatal("an overlong identity was catalogued")
	}
}

// A post counted under a filter always has a source under it: with two rows of
// one resource in a message, the source is the first row that matches.
func TestLinkCatalogSourcesFollowTheFilter(t *testing.T) {
	f := newLinkFixture(t)
	text := ""
	for i := range 11 {
		text += fmt.Sprintf("https://i.test/x%d.png ", i)
	}
	// Catalog classification no longer depends on the retired ten-image gallery.
	text += "https://I.test/x0.png"
	f.postAs(t, f.dev, "one:m1", "peer", text, time.Unix(9000, 0).UTC(), nil)
	page := f.links(t, "alice", LinkQuery{Kinds: []string{"link"}})
	if len(page.Items) != 0 {
		t.Fatalf("link kind: %+v", page.Items)
	}
	if page = f.links(t, "alice", LinkQuery{Kinds: []string{"image"}}); len(page.Items) != 11 {
		t.Fatalf("image kind: %d cards", len(page.Items))
	}
	for _, card := range page.Items {
		if card.Count != 1 || len(card.Sources) != 1 {
			t.Fatalf("source count: %+v", card)
		}
	}
}

func TestLinkCatalogHostFilterAcceptsHostsAndAddresses(t *testing.T) {
	f := newLinkFixture(t)
	f.postAs(t, f.dev, "one:m1", "peer", "https://[::1]:8443/x http://127.0.0.1/y", time.Unix(9100, 0).UTC(), nil)
	for host, want := range map[string]int{"::1": 1, "[0:0::1]": 1, "127.0.0.1": 1, "example.com": 0} {
		if got := f.links(t, "alice", LinkQuery{Host: host}); len(got.Items) != want {
			t.Errorf("host %q: %d cards, want %d", host, len(got.Items), want)
		}
	}
	for _, bad := range []string{"a/b", "user@a.test", "a b", "a.test:80", "?#"} {
		if _, err := f.m.Links("alice", LinkQuery{Limit: 30, Host: bad}); err == nil {
			t.Errorf("host %q accepted", bad)
		}
	}
}

func TestLinkCatalogSaysWhenHistoryIsStillIndexed(t *testing.T) {
	f := newLinkFixture(t)
	if _, err := f.store.DB.Exec("DELETE FROM index_jobs"); err != nil {
		t.Fatal(err)
	}
	if page := f.links(t, "alice", LinkQuery{}); !page.Indexing {
		t.Fatal("no indexing hint before the backfill")
	}
	for {
		done, err := f.store.BackfillLinksStep()
		if err != nil {
			t.Fatal(err)
		}
		if done {
			break
		}
	}
	if page := f.links(t, "alice", LinkQuery{}); page.Indexing {
		t.Fatal("indexing hint after the backfill")
	}
}
