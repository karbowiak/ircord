package storage

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/msgindex"
)

func TestScopedSearchCursorMaximumEscapedID(t *testing.T) {
	for _, id := range []string{strings.Repeat("\x00", 200), strings.Repeat("\\", 200), strings.Repeat("\"", 200)} {
		msg := model.Message{ID: id, CreatedAt: time.Unix(0, -9223372036854775807).UTC()}
		scope := strings.Repeat("a", 32)
		cursor := ScopedSearchCursor(msg, scope, "capability-unavailable")
		if len(cursor) > 512 {
			t.Fatalf("cursor length %d", len(cursor))
		}
		at, decoded, reason, err := CheckSearchCursor(cursor, scope)
		if err != nil || decoded != id || at != msg.CreatedAt.UnixNano() || reason != "capability-unavailable" {
			t.Fatalf("cursor round trip: %q %q %v", decoded, reason, err)
		}
	}
}

func TestSearchFiltersPagingAndMemoryParity(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	account(t, s, "bob")
	at := time.Unix(1000, 0).UTC()
	messages := []model.Message{
		{ID: "a", ConversationID: "one", SenderID: "human", Text: "Hello @Jul, world café", CreatedAt: at, Kind: "message"},
		{ID: "b", ConversationID: "two", SenderID: "bot", Text: "Hello Jul-else https://x.test/a.png", CreatedAt: at, Kind: "message"},
		{ID: "c", ConversationID: "one", SenderID: "bot", Text: "Hello @Jul! https://tenor.com/view/cat", CreatedAt: at, Kind: "message"},
		{ID: "d", ConversationID: "two", SenderID: "human", Text: "Hello @Jul https://vimeo.com/123", CreatedAt: at, Kind: "message"},
		{ID: "e", ConversationID: "one", SenderID: "human", Text: "Hello @Jul", CreatedAt: at, Kind: "message", Attachment: &model.Attachment{ContentType: "application/pdf"}},
		{ID: "f", ConversationID: "one", SenderID: "human", Text: "Hello code \x11https://x.test/a.gif\x11", CreatedAt: at, Kind: "message", Attachment: &model.Attachment{ContentType: "image/gif"}},
		{ID: "g", ConversationID: "one", SenderID: "human", Text: "Hello", CreatedAt: at, Kind: "message", Attachment: &model.Attachment{ContentType: "image/png"}},
		{ID: "h", ConversationID: "two", SenderID: "bot", Text: "Hello", CreatedAt: at, Kind: "message", Attachment: &model.Attachment{ContentType: "video/mp4"}},
		{ID: "i", ConversationID: "one", SenderID: "human", Text: "Hello", CreatedAt: at, Kind: "message", GIF: &model.GIF{}},
		{ID: "z", ConversationID: "one", SenderID: "human", Text: "Hello @Jul", CreatedAt: at, Kind: "message", Deleted: true},
	}
	for _, msg := range messages {
		if err = s.SaveHistory("alice", msg); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.SaveHistory("bob", model.Message{ID: "foreign", ConversationID: "one", SenderID: "human", Text: "Hello @Jul", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		filter SearchFilter
		want   string
	}{
		{"text", SearchFilter{Query: "hel"}, "i,h,g,f,e,d,c,b,a"},
		{"phrase", SearchFilter{Query: "world caf"}, "a"},
		{"diacritics", SearchFilter{Query: "cafe"}, "a"},
		{"middle substring", SearchFilter{Query: "orl"}, ""},
		{"from", SearchFilter{From: []string{"bot"}}, "h,c,b"},
		{"from OR", SearchFilter{From: []string{"bot", "human"}}, "i,h,g,f,e,d,c,b,a"},
		{"conversations", SearchFilter{Conversations: []string{"one", "two"}}, "i,h,g,f,e,d,c,b,a"},
		{"mentions", SearchFilter{Mentions: []string{"Jul"}}, "e,d,c,a"},
		{"mention OR", SearchFilter{Mentions: []string{"nobody", "Jul"}}, "e,d,c,a"},
		{"links", SearchFilter{Has: []string{"link"}}, "d,c,b"},
		{"images", SearchFilter{Has: []string{"image"}}, "g,b"},
		{"gifs", SearchFilter{Has: []string{"gif"}}, "i,f,c"},
		{"videos", SearchFilter{Has: []string{"video"}}, "h,d"},
		{"files", SearchFilter{Has: []string{"file"}}, "e"},
		{"has OR", SearchFilter{Has: []string{"image", "file"}}, "g,e,b"},
		{"before exclusive", SearchFilter{Before: at}, ""},
		{"after inclusive", SearchFilter{After: at}, "i,h,g,f,e,d,c,b,a"},
		{"bot", SearchFilter{Author: "bot", Bots: []string{"bot"}}, "h,c,b"},
		{"user", SearchFilter{Author: "user", Bots: []string{"bot"}}, "i,g,f,e,d,a"},
		{"no bots", SearchFilter{Author: "bot"}, ""},
		{"combined", SearchFilter{Query: "hello", From: []string{"bot"}, Conversations: []string{"one"}, Mentions: []string{"Jul"}, Has: []string{"gif"}, After: at, Before: at.Add(time.Second), Author: "bot", Bots: []string{"bot"}}, "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ids := []string{}
			cursor := ""
			for {
				page, e := s.SearchPage("alice", tc.filter, cursor, 2)
				if e != nil {
					t.Fatal(e)
				}
				for _, msg := range page.Items {
					ids = append(ids, msg.ID)
				}
				if page.NextCursor == nil {
					break
				}
				cursor = *page.NextCursor
				if len(ids) > len(messages) {
					t.Fatal("pagination did not advance")
				}
			}
			if strings.Join(ids, ",") != tc.want {
				t.Fatalf("want %s, got %v", tc.want, ids)
			}
			memory := []string{}
			for i := len(messages) - 1; i >= 0; i-- {
				if MatchesSearch(messages[i], tc.filter) {
					memory = append(memory, messages[i].ID)
				}
			}
			if !reflect.DeepEqual(ids, memory) {
				t.Fatalf("memory mismatch: %v, SQL %v", memory, ids)
			}
		})
	}
}

func TestSearchEditsDeletesAndLiteralSyntax(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	msg := model.Message{ID: "m", ConversationID: "c", Text: "NEAR old-world café शब्द й", CreatedAt: time.Unix(1000, 0), Kind: "message"}
	if err = s.SaveHistory("alice", msg); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`"NEAR"`, `NEAR`, `old-*`, `-world`, `"`, `*`, `-`, "शब्द", "й"} {
		page, e := s.SearchPage("alice", SearchFilter{Query: q}, "", 10)
		if e != nil {
			t.Fatalf("%q: %v", q, e)
		}
		want := msgindex.SearchPhrase(msg.Text, q)
		if (len(page.Items) == 1) != want {
			t.Fatalf("literal %q: %+v", q, page)
		}
	}
	msg.Text = "replacement"
	if err = s.SaveHistory("alice", msg); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"old", "replacement"} {
		page, e := s.SearchPage("alice", SearchFilter{Query: q}, "", 10)
		if e != nil || (len(page.Items) == 1) != (q == "replacement") {
			t.Fatalf("edit %s: %+v %v", q, page, e)
		}
	}
	msg.Deleted = true
	if err = s.SaveHistory("alice", msg); err != nil {
		t.Fatal(err)
	}
	page, err := s.SearchPage("alice", SearchFilter{Query: "replacement"}, "", 10)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("deleted: %+v %v", page, err)
	}
	msg.Deleted = false
	if err = s.SaveHistory("alice", msg); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("DELETE FROM messages WHERE owner=?", "alice"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = s.DB.QueryRow("SELECT count(*) FROM message_search").Scan(&n); err != nil || n != 0 {
		t.Fatal("orphan FTS row", n, err)
	}
}

func TestSearchBackfillResumesAndQueryPlans(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	account(t, s, "alice")
	for i := 0; i < 5; i++ {
		if err = s.SaveHistory("alice", model.Message{ID: fmt.Sprint(i), ConversationID: "room", SenderID: "peer", Text: "retained https://x.test/a.gif", CreatedAt: time.Unix(1000+int64(i), 0), Kind: "message"}); err != nil {
			t.Fatal(err)
		}
	}
	// Simulate an older complete link index, before the search columns existed.
	if _, err = s.DB.Exec("UPDATE messages SET search_visible=0,search_kinds=0"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT OR REPLACE INTO index_jobs(name,version,last_rowid,done,updated_ns) VALUES(?,3,5,1,0)", linkBackfill); err != nil {
		t.Fatal(err)
	}
	previous := LinkBackfillBatch
	LinkBackfillBatch = 1
	defer func() { LinkBackfillBatch = previous }()
	if done, e := s.BackfillLinksStep(); e != nil || done {
		t.Fatal(done, e)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; ; i++ {
		done, e := s.BackfillLinksStep()
		if e != nil {
			t.Fatal(e)
		}
		if done {
			break
		}
		if i > 8 {
			t.Fatal("backfill did not finish")
		}
	}
	page, err := s.SearchPage("alice", SearchFilter{Query: "retained", Has: []string{"gif"}}, "", 10)
	if err != nil || len(page.Items) != 5 {
		t.Fatal(page, err)
	}
	for name, f := range map[string]SearchFilter{"text": {Query: "retained"}, "from": {From: []string{"peer"}}, "date": {After: time.Unix(1000, 0)}, "has": {Has: []string{"gif"}}, "conversation": {Conversations: []string{"room"}}} {
		query, args, e := searchSQL("alice", f, "")
		if e != nil {
			t.Fatal(e)
		}
		rows, e := s.DB.Query("EXPLAIN QUERY PLAN "+query, args...)
		if e != nil {
			t.Fatal(e)
		}
		plan := []string{}
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if e = rows.Scan(&id, &parent, &unused, &detail); e != nil {
				t.Fatal(e)
			}
			plan = append(plan, detail)
		}
		rows.Close()
		joined := strings.Join(plan, "; ")
		t.Logf("%s: %s", name, joined)
		if strings.Contains(joined, "SCAN m ") || !strings.Contains(joined, "SEARCH m USING") {
			t.Fatalf("unindexed messages query: %s", joined)
		}
		if name == "text" && !strings.Contains(joined, "VIRTUAL TABLE INDEX") {
			t.Fatal("text did not use FTS", joined)
		}
		if name == "text" && !strings.Contains(joined, "SEARCH m USING INTEGER PRIMARY KEY") {
			t.Fatal("text walks account messages instead of FTS hits", joined)
		}
	}
}
