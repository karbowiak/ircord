package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

// Deleting a message runs these lookups once per receiving account. Without
// planner statistics an unindexed lookup reads every retained event of the
// account, so each must search its own partial index.
func TestDeletionLookupsStayIndexedWithoutStatistics(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	at := time.Now().UTC()
	for i := range 200 {
		msg := model.Message{ID: fmt.Sprintf("m%d", i), UpstreamID: fmt.Sprintf("u%d", i), ConversationID: "room", SenderID: "bob", Kind: "message", Text: "hello", CreatedAt: at}
		if i%2 == 1 {
			msg.ReplyTo = &model.Reply{ID: fmt.Sprintf("m%d", i-1), Nick: "bob", Text: "hello"}
		}
		if _, err = s.Commit("alice", "message.created", msg); err != nil {
			t.Fatal(err)
		}
		line := model.ServerLine{ID: fmt.Sprintf("l%d", i), NetworkID: "n", Command: "PRIVMSG", Params: []string{"#room", "hello"}, Text: "hello", Tags: map[string]string{"msgid": msg.UpstreamID}}
		if _, err = s.Commit("alice", "server.line", line); err != nil {
			t.Fatal(err)
		}
	}
	type planCheck struct {
		name, query, want string
		args              []any
	}
	checks := []planCheck{
		{"replies", repliesQuery, "messages_replies", []any{"alice", "room", "m0"}},
		{"message events", redactedEventsUpdate, "events_message", []any{"{}", "alice", "m0"}},
	}
	for _, lookup := range append(append(append([]serverLineLookup{}, lineByMsgid...), lineByClient...), lineInBatch, lineBatchRef) {
		checks = append(checks,
			planCheck{lookup.index + " (one value)", lookup.query(1), lookup.index, []any{"alice", "u1", "n"}},
			planCheck{lookup.index + " (two values)", lookup.query(2), lookup.index, []any{"alice", "u1", "u2", "n"}},
		)
	}
	for _, check := range checks {
		plan := queryPlan(t, s, check.query, check.args...)
		if !strings.Contains(plan, "USING INDEX "+check.want) || strings.Contains(plan, "SCAN") {
			t.Errorf("%s plan = %q, want a search using %s", check.name, plan, check.want)
		}
	}
	replies, err := s.Replies("alice", "room", "m10")
	if err != nil || len(replies) != 1 || replies[0].ID != "m11" {
		t.Fatalf("Replies(m10) = %+v, %v; want m11", replies, err)
	}
}

// The replay log lookup must scrub exactly the lines that scrubbing the whole
// replay log would: matches by every tag, whole multiline batches found from
// their opening line or from one member, and nothing on other networks.
func TestScrubRedactedChangesSameLinesAsFullScan(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	attachment := map[string]string{"+ircord/attachment": "file"}
	tags := func(base map[string]string, extra ...map[string]string) map[string]string {
		out := map[string]string{}
		for _, m := range append([]map[string]string{base}, extra...) {
			for k, v := range m {
				out[k] = v
			}
		}
		return out
	}
	privmsg := func(id, network, text string, tags map[string]string) model.ServerLine {
		return model.ServerLine{ID: id, NetworkID: network, Command: "PRIVMSG", Params: []string{"#room", text}, Text: text, Tags: tags}
	}
	batch := func(id, network, ref string, tags map[string]string) model.ServerLine {
		params := []string{ref}
		if strings.HasPrefix(ref, "+") {
			params = append(params, "draft/multiline", "#room")
		}
		return model.ServerLine{ID: id, NetworkID: network, Command: "BATCH", Params: params, Tags: tags}
	}
	lines := []model.ServerLine{
		// The message as received: a multiline batch opened with its msgid.
		batch("in-open", "n", "+in", tags(map[string]string{"msgid": "up"}, attachment)),
		privmsg("in-1", "n", "secret one", map[string]string{"batch": "in"}),
		privmsg("in-2", "n", "secret two", tags(map[string]string{"batch": "in"}, attachment)),
		batch("in-close", "n", "-in", nil),
		// The author's outgoing copy: only a member carries the client id.
		batch("out-open", "n", "+out", attachment),
		privmsg("out-1", "n", "secret one", map[string]string{"batch": "out", "+ircord/client-id": "client-1"}),
		privmsg("out-2", "n", "secret two", map[string]string{"batch": "out"}),
		batch("out-close", "n", "-out", nil),
		// The labelled echo, an edit by msgid and an edit's own outgoing line.
		privmsg("echo", "n", "secret", map[string]string{"label": "client-1"}),
		privmsg("edit-in", "n", "secret edited", map[string]string{"msgid": "edit-1"}),
		privmsg("edit-out", "n", "secret edited", map[string]string{"+ircord/edit": "up"}),
		// Lines that must stay: other messages, another network, a batch that is not multiline.
		privmsg("other", "n", "keep", map[string]string{"msgid": "up-other"}),
		batch("keep-open", "n", "+keep", attachment),
		privmsg("keep-1", "n", "keep", map[string]string{"batch": "keep"}),
		batch("keep-close", "n", "-keep", nil),
		privmsg("elsewhere", "m", "keep", map[string]string{"msgid": "up"}),
		{ID: "plain-batch", NetworkID: "n", Command: "BATCH", Params: []string{"+chat", "chathistory", "#room"}, Tags: map[string]string{"msgid": "up"}},
		privmsg("plain-1", "n", "keep", map[string]string{"batch": "chat"}),
	}
	for _, line := range lines {
		if _, err = s.Commit("alice", "server.line", line); err != nil {
			t.Fatal(err)
		}
	}
	// What scrubbing every line of the network gives, as the lookup did before.
	want := map[string]model.ServerLine{}
	for _, network := range []string{"n", "m"} {
		all := []model.ServerLine{}
		for _, line := range lines {
			if line.NetworkID == network {
				raw, _ := json.Marshal(line)
				var copy model.ServerLine
				_ = json.Unmarshal(raw, &copy)
				all = append(all, copy)
			}
		}
		if network == "n" {
			ScrubServerLines(all, "up", "client-1", "Message deleted")
			ScrubServerLines(all, "edit-1", "", "Message deleted")
		}
		for _, line := range all {
			want[line.ID] = line
		}
	}
	deleted := model.Message{ID: "n:up", UpstreamID: "up", ConversationID: "room", Kind: "message", Text: "Message deleted", Deleted: true}
	if err = s.ScrubRedacted("alice", "n", deleted, "client-1", "edit-1"); err != nil {
		t.Fatal(err)
	}
	events, _, err := s.Replay("alice", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range events {
		if e.Type != "server.line" {
			continue
		}
		var got model.ServerLine
		if err = json.Unmarshal(e.Data, &got); err != nil {
			t.Fatal(err)
		}
		seen++
		gotRaw, _ := json.Marshal(got)
		wantRaw, _ := json.Marshal(want[got.ID])
		if string(gotRaw) != string(wantRaw) {
			t.Errorf("line %s = %+v, want %+v", got.ID, got, want[got.ID])
		}
	}
	if seen != len(lines) {
		t.Fatalf("replayed %d server lines, want %d", seen, len(lines))
	}
	for _, id := range []string{"in-open", "in-1", "in-2", "out-open", "out-1", "out-2", "echo", "edit-in", "edit-out"} {
		if line := want[id]; strings.Contains(line.Text, "secret") || line.Tags["+ircord/attachment"] != "" {
			t.Errorf("reference scrub kept %s: %+v", id, line)
		}
	}
	for _, id := range []string{"other", "keep-open", "keep-1", "elsewhere", "plain-1"} {
		if line := want[id]; line.Text == "Message deleted" {
			t.Errorf("reference scrub changed %s", id)
		}
	}
}
