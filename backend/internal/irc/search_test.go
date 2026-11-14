package irc

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

func TestManagerSearchFiltersAndFallback(t *testing.T) {
	for _, durable := range []bool{true, false} {
		t.Run(map[bool]string{true: "storage", false: "memory"}[durable], func(t *testing.T) {
			f := newLinkFixture(t)
			w := f.m.space("alice")
			w.mu.Lock()
			w.members["peer"] = Member{ID: "peer", Nick: "Jul", Role: "member"}
			w.members["bot"] = Member{ID: "bot", Nick: "Helper", Role: "bot"}
			w.mu.Unlock()
			at := time.Unix(2000, 0).UTC()
			for _, id := range []string{"a", "b", "c", "d", "e"} {
				f.postAs(t, f.dev, id, "bot", "hello @Jul https://x.test/a.png", at, nil)
			}
			f.postAs(t, f.other, "z", "peer", "hello @Jul", at, nil)
			if !durable {
				// emit persists events; receive normally maintains the memory cache.
				dev, e := f.store.History("alice", f.dev.ID, "", 100)
				if e != nil {
					t.Fatal(e)
				}
				other, e := f.store.History("alice", f.other.ID, "", 100)
				if e != nil {
					t.Fatal(e)
				}
				w.mu.Lock()
				w.messages[f.dev.ID] = dev.Items
				w.messages[f.other.ID] = other.Items
				w.store = nil
				w.mu.Unlock()
			}
			q := SearchQuery{From: []string{"bot", "foreign"}, Mentions: []string{"peer"}, Has: []string{"image"}, Author: "bot", After: at, Before: at.Add(time.Second), Conversations: []string{f.dev.ID, f.other.ID}, Limit: 2}
			ids := []string{}
			for {
				page, err := f.m.SearchMessages("alice", q)
				if err != nil {
					t.Fatal(err)
				}
				for _, r := range page.Items {
					ids = append(ids, r.Message.ID)
				}
				if page.NextCursor == nil {
					break
				}
				q.Cursor = *page.NextCursor
				if len(ids) > 5 {
					t.Fatal("pagination loop")
				}
			}
			if !reflect.DeepEqual(ids, []string{"e", "d", "c", "b", "a"}) {
				t.Fatal(ids)
			}
			for _, tc := range []struct {
				name  string
				query SearchQuery
				want  string
			}{
				{"from", SearchQuery{From: []string{"bot"}}, "e,d,c,b,a"},
				{"mentions", SearchQuery{Mentions: []string{"peer"}}, "z,e,d,c,b,a"},
				{"conversation", SearchQuery{Conversations: []string{f.dev.ID}}, "e,d,c,b,a,one:abc"},
				{"has", SearchQuery{Has: []string{"image"}}, "e,d,c,b,a"},
				{"after", SearchQuery{After: at}, "z,e,d,c,b,a"},
				{"before", SearchQuery{Before: at}, "two:abc,one:abc"},
				{"bot", SearchQuery{Author: "bot"}, "e,d,c,b,a"},
				{"user", SearchQuery{Author: "user"}, "z,two:abc,one:abc"},
				{"text", SearchQuery{Query: "hel"}, "z,e,d,c,b,a"},
			} {
				page, err := f.m.SearchMessages("alice", tc.query)
				if err != nil {
					t.Fatal(err)
				}
				got := []string{}
				for _, item := range page.Items {
					got = append(got, item.Message.ID)
				}
				if strings.Join(got, ",") != tc.want {
					t.Fatalf("%s: got %v, want %s", tc.name, got, tc.want)
				}
			}
			for _, q := range []SearchQuery{{From: []string{"foreign"}}, {Mentions: []string{"foreign"}}, {Author: "bot", From: []string{"peer"}}} {
				page, err := f.m.SearchMessages("alice", q)
				if err != nil || len(page.Items) != 0 {
					t.Fatal(page, err)
				}
			}
			page, err := f.m.SearchMessages("bob", SearchQuery{Query: "hello"})
			if err != nil || len(page.Items) != 0 {
				t.Fatal("account leak", page, err)
			}
			if _, err = f.m.SearchMessages("bob", SearchQuery{Conversations: []string{f.dev.ID}}); !errors.Is(err, ErrNotFound) {
				t.Fatal(err)
			}
			for _, q := range []SearchQuery{{}, {Query: "x"}, {Query: strings.Repeat("x", 201)}, {Has: []string{"invalid"}}, {Author: "invalid"}, {From: make([]string, 51)}, {Before: at, After: at}, {Has: []string{"link"}, Limit: 101}, {Query: "hello", Cursor: "broken"}} {
				if _, err = f.m.SearchMessages("alice", q); err == nil {
					t.Fatalf("invalid search accepted: %+v", q)
				}
			}
			// A one-character query is valid with a filter.
			page, err = f.m.SearchMessages("alice", SearchQuery{Query: "h", Author: "bot"})
			if err != nil || len(page.Items) != 5 {
				t.Fatal(page, err)
			}
		})
	}
}

func TestSearchCursorScopeAndFallbackContinuation(t *testing.T) {
	f := newLinkFixture(t)
	at := time.Unix(2000, 0).UTC()
	for _, id := range []string{"a", "b", "c"} {
		f.postAs(t, f.dev, id, "peer", "hello world", at, nil)
	}
	q := SearchQuery{Query: "HELLO, wor", Conversations: []string{f.dev.ID}, Source: "upstream", Limit: 1}
	first, err := f.m.SearchMessages("alice", q)
	if err != nil || first.NextCursor == nil || first.Source == nil || first.Source.Actual != "retained" {
		t.Fatalf("first: %+v %v", first, err)
	}
	q.Cursor, q.Query = *first.NextCursor, "hello wor"
	second, err := f.m.SearchMessages("alice", q)
	if err != nil || second.Source.Actual != "retained" || second.Source.Reason != first.Source.Reason || second.Items[0].Message.ID == first.Items[0].Message.ID {
		t.Fatalf("continuation: %+v %v", second, err)
	}
	q.Query = "different"
	if _, err = f.m.SearchMessages("alice", q); !errors.Is(err, storage.ErrInvalidCursor) {
		t.Fatalf("changed query: %v", err)
	}
	q.Query = "hello wor"
	if _, err = f.m.SearchMessages("bob", q); !errors.Is(err, storage.ErrInvalidCursor) {
		t.Fatalf("changed owner: %v", err)
	}
	legacy := storage.SearchCursor(first.Items[0].Message)
	q.Cursor = legacy
	if _, err = f.m.SearchMessages("alice", q); err != nil {
		t.Fatalf("legacy cursor: %v", err)
	}
}
