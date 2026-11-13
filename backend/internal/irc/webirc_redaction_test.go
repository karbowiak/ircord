package irc

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
)

func TestWebIRCReceivePreservesContentAndHistoryIdentifiers(t *testing.T) {
	f := historyFixture(t)
	c := f.c
	private := []string{"fixture-secret", "127.0.0.1", "127.0.0.1"}
	c.privateRedaction.Store(&private)
	req := &historyRequest{label: "label-fixture-secret", sub: "AFTER", target: "#test", done: make(chan historyResult, 1)}
	c.mu.Lock()
	c.historyRequests[req.label] = req
	c.mu.Unlock()
	open := girc.Event{Command: "BATCH", Params: []string{"+batch-fixture-secret", "chathistory", "#test"}, Tags: girc.Tags{"label": req.label, "vendor/fixture-secret": "127.0.0.1"}}
	before := open.Copy()
	c.receiveFromAttempt(c.client, open)
	if !reflect.DeepEqual(open, *before) {
		t.Fatal("batch callback mutated the caller's event")
	}
	c.mu.Lock()
	matched := c.historyResponses["batch-fixture-secret"] == req && c.batchLabels["batch-fixture-secret"] == req.label
	c.mu.Unlock()
	if !matched {
		t.Fatal("batch reference or label changed before history matching")
	}
	text := "Open http://127.0.0.1:8080/media/fixture-secret.png"
	msgID := "id-fixture-secret-127.0.0.1"
	account := "account-fixture-secret"
	event := girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "nick-fixture-secret", Ident: "user-fixture-secret", Host: "127.0.0.1"}, Params: []string{"#test", text}, Tags: girc.Tags{"batch": "batch-fixture-secret", "msgid": msgID, "account": account, "label": "label-fixture-secret", "time": f.at.Format(time.RFC3339Nano)}}
	before = event.Copy()
	c.receiveFromAttempt(c.client, event)
	if !reflect.DeepEqual(event, *before) {
		t.Fatal("message callback mutated Params, Tags, or Source")
	}
	conv := c.conversationID("#test")
	page, err := f.store.History("alice", conv, "", 40)
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("stored history: %v, %v", page.Items, err)
	}
	message := page.Items[0]
	if message.Text != text || message.UpstreamID != msgID || message.SenderAccount != account {
		t.Fatalf("message data changed: %+v", message)
	}
	c.mu.Lock()
	count, retainedID := req.result.Count, req.result.IDs[msgID]
	c.mu.Unlock()
	if count != 1 || !retainedID {
		t.Fatal("history did not match the original message ID")
	}
	c.owner.mu.Lock()
	sender := c.owner.members[message.SenderID]
	trace := append([]ServerLine{}, c.owner.trace["net"]...)
	c.owner.mu.Unlock()
	if len(trace) < 2 {
		t.Fatal("expected batch and message diagnostic entries")
	}
	if sender.Nick != "nick-fixture-secret" || sender.Account != account || sender.Username != "user-fixture-secret" || sender.Host != "127.0.0.1" {
		t.Fatalf("sender identity changed: %+v", sender)
	}
	for _, line := range trace {
		if strings.Contains(line.Text, "fixture-secret") || strings.Contains(line.Text, "127.0.0.1") || strings.Contains(line.Source, "fixture-secret") || strings.Contains(line.Source, "127.0.0.1") {
			t.Fatal("diagnostic trace exposed a private value")
		}
		for key, value := range line.Tags {
			if strings.Contains(key, "fixture-secret") || strings.Contains(value, "fixture-secret") || strings.Contains(value, "127.0.0.1") {
				t.Fatal("diagnostic tag exposed a private value")
			}
		}
	}
}

func TestWebIRCDiagnosticAndCommandErrorsAreRedacted(t *testing.T) {
	f := historyFixture(t)
	c := f.c
	private := []string{"fixture-secret", "127.0.0.1", "127.0.0.1"}
	c.privateRedaction.Store(&private)
	c.owner.mu.Lock()
	c.owner.info["net"] = newConnectionInfo("net")
	c.owner.networks["net"] = Network{ID: "net", ISupport: map[string]string{}}
	c.owner.mu.Unlock()
	_, events, _, unsubscribe := f.m.Subscribe("alice", 0)
	defer unsubscribe()
	welcome := girc.Event{Command: "001", Source: &girc.Source{Name: "server-fixture-secret"}, Params: []string{"alice", "welcome fixture-secret from 127.0.0.1"}, Tags: girc.Tags{"test": "fixture-secret"}}
	before := welcome.Copy()
	c.receiveFromAttempt(c.client, welcome)
	if !reflect.DeepEqual(welcome, *before) {
		t.Fatal("welcome callback changed the input event")
	}
	url := "http://127.0.0.1/fixture-secret"
	support := girc.Event{Command: "005", Source: &girc.Source{Name: "server"}, Params: []string{"alice", "draft/FILEHOST=" + url, "X-fixture-secret=value", "supported"}}
	c.receiveFromAttempt(c.client, support)
	c.owner.mu.Lock()
	gotURL := c.owner.networks["net"].ISupport["draft/FILEHOST"]
	gotKey := c.owner.networks["net"].ISupport["X-fixture-secret"]
	info := c.owner.info["net"]
	trace := append([]ServerLine{}, c.owner.trace["net"]...)
	c.owner.mu.Unlock()
	if len(trace) < 2 {
		t.Fatal("expected welcome and ISUPPORT diagnostic entries")
	}
	if gotURL != url || gotKey != "value" {
		t.Fatalf("ISUPPORT semantics changed: %q", gotURL)
	}
	for key := range info.ISupport {
		if strings.Contains(key, "fixture-secret") {
			t.Fatal("diagnostic ISUPPORT key exposed private data")
		}
	}
	if strings.Contains(info.ISupport["draft/FILEHOST"], "fixture-secret") || strings.Contains(info.ISupport["draft/FILEHOST"], "127.0.0.1") {
		t.Fatal("diagnostic ISUPPORT exposed private data")
	}
	for _, line := range trace {
		if strings.Contains(line.Source, "fixture-secret") || strings.Contains(line.Text, "fixture-secret") || strings.Contains(line.Text, "127.0.0.1") {
			t.Fatal("welcome or diagnostic trace exposed private data")
		}
	}
	failure := girc.Event{Command: "FAIL", Source: &girc.Source{Name: "server"}, Params: []string{"PRIVMSG", "FAILED", "failure fixture-secret at 127.0.0.1"}, Tags: girc.Tags{"label": "raw:client-fixture-secret"}}
	before = failure.Copy()
	c.receiveFromAttempt(c.client, failure)
	if !reflect.DeepEqual(failure, *before) {
		t.Fatal("error callback changed the input event")
	}
	for {
		select {
		case event := <-events:
			if event.Type != "command.error" {
				continue
			}
			if event.ClientID != "client-fixture-secret" {
				t.Fatalf("command label changed: %q", event.ClientID)
			}
			if strings.Contains(event.Message, "fixture-secret") || strings.Contains(event.Message, "127.0.0.1") || !strings.Contains(event.Message, "[redacted]") {
				t.Fatalf("unsafe command error: %q", event.Message)
			}
			return
		case <-time.After(time.Second):
			t.Fatal("command error was not emitted")
		}
	}
}

func TestWebIRCOwnHostDisplayDoesNotRewriteOtherHosts(t *testing.T) {
	c := &connection{}
	private := []string{"fixture-secret", "0::1", "::1"}
	c.privateRedaction.Store(&private)
	if got := c.memberHostForDisplay(true, "0:0:0:0:0:0:0:1"); got != "[redacted]" {
		t.Fatalf("own origin displayed as %q", got)
	}
	for _, host := range []string{"0:0:0:0:0:0:0:1", "fixture-secret.example"} {
		if got := c.memberHostForDisplay(false, host); got != host {
			t.Fatalf("other host changed to %q", got)
		}
	}
	if got := c.memberHostForDisplay(true, "fixture-secret.example"); got != "fixture-secret.example" {
		t.Fatalf("non-address host changed to %q", got)
	}
}

func TestWebIRCDiagnosticHistoryIDsStayDistinctAndRedacted(t *testing.T) {
	f := historyFixture(t)
	c := f.c
	private := []string{"fixture-secret", "127.0.0.1", "127.0.0.1"}
	c.privateRedaction.Store(&private)
	for _, id := range []string{"_fixture-secret-A", "_fixture-secret-A", "_fixture-secret-B"} {
		event := girc.Event{Command: "NOTICE", Source: &girc.Source{Name: "HistServ"}, Params: []string{"#test", "history status"}, Tags: girc.Tags{"msgid": id, "time": f.at.Format(time.RFC3339Nano)}}
		before := event.Copy()
		c.receiveFromAttempt(c.client, event)
		if !reflect.DeepEqual(event, *before) {
			t.Fatal("history status callback changed the message ID")
		}
	}
	c.owner.mu.Lock()
	lines := append([]ServerLine{}, c.owner.serverLogs["net"]...)
	c.owner.mu.Unlock()
	if len(lines) != 2 || lines[0].ID == lines[1].ID {
		t.Fatalf("history status deduplication failed: %+v", lines)
	}
	for _, line := range lines {
		if strings.Contains(line.ID, "fixture-secret") {
			t.Fatal("diagnostic line ID exposed the credential")
		}
	}
	page, err := f.store.History("alice", c.conversationID("#test"), "", 40)
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("history messages: %+v, %v", page.Items, err)
	}
	ids := map[string]bool{}
	for _, message := range page.Items {
		ids[message.UpstreamID] = true
	}
	if !ids["_fixture-secret-A"] || !ids["_fixture-secret-B"] {
		t.Fatalf("stored history IDs changed: %+v", ids)
	}
}

func TestWebIRCGircMemberSyncPreservesAccountIdentity(t *testing.T) {
	port, peers := capabilityServer(t, "extended-join", "")
	m := New(Options{AllowPrivate: true})
	defer m.Close()
	if _, err := m.Connect("alice", NetworkConfig{ID: "net", Name: "Test", Host: "127.0.0.1", Port: port, Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	c, err := m.connection("alice", "net")
	if err != nil {
		t.Fatal(err)
	}
	private := []string{"fixture-secret", "127.0.0.1", "127.0.0.1"}
	c.privateRedaction.Store(&private)
	peer := <-peers
	peer.send(":nick-fixture-secret!user-fixture-secret@127.0.0.1 JOIN #test account-fixture-secret :name-fixture-secret")
	eventually(t, func() bool {
		c.owner.mu.Lock()
		defer c.owner.mu.Unlock()
		for _, member := range c.owner.members {
			if member.Nick == "nick-fixture-secret" {
				return member.Account == "account-fixture-secret" && member.Username == "user-fixture-secret" && member.Host == "127.0.0.1" && member.Realname == "name-fixture-secret"
			}
		}
		return false
	})
}
