package irc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func setMemberModes(c *connection, conv Conversation, modes map[string]string) {
	c.owner.mu.Lock()
	defer c.owner.mu.Unlock()
	v := c.owner.conversations[conv.ID]
	v.MemberModes = modes
	c.owner.conversations[conv.ID] = v
}

func TestRedactAsChannelOperatorDeletesOthersMessages(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{"message-tags": true, "draft/message-redaction": true})
	ctx := context.Background()
	if _, err := m.RedactAs(ctx, "alice", conv.ID, "network:m2", ""); !errors.Is(err, ErrDeleteForbidden) || !errors.Is(err, ErrForbidden) {
		t.Fatalf("a member without operator status deleted someone else's message: %v", err)
	}
	// Ergo lets only @ and above delete other people's messages.
	setMemberModes(c, conv, map[string]string{"alice": "%"})
	if _, err := m.RedactAs(ctx, "alice", conv.ID, "network:m2", ""); !errors.Is(err, ErrDeleteForbidden) {
		t.Fatalf("a half-operator deleted someone else's message: %v", err)
	}
	if len(c.queue) != 0 {
		t.Fatal("a refused deletion reached the network")
	}
	setMemberModes(c, conv, map[string]string{"alice": "@"})
	r, err := m.RedactAs(ctx, "alice", conv.ID, "network:m2", " spam ")
	if err != nil {
		t.Fatal(err)
	}
	if r.Authority != AuthorityOperator || !r.Message.Deleted || r.Original.Text != "" || r.Original.UpstreamID != "m2" || r.Sender != "Peer" {
		t.Fatalf("result: %+v", r)
	}
	sent := <-c.queue
	if got := strings.Join(sent[0].Params, " "); sent[0].Command != "REDACT" || got != "#test m2 spam" {
		t.Fatalf("sent %s %s", sent[0].Command, got)
	}
	// The author's own deletion sends no reason.
	r, err = m.RedactAs(ctx, "alice", conv.ID, "network:m1", "")
	if err != nil || r.Authority != AuthorityAuthor {
		t.Fatalf("own deletion: %v %+v", err, r)
	}
	if sent = <-c.queue; len(sent[0].Params) != 2 {
		t.Fatalf("own deletion params %v", sent[0].Params)
	}
	// Operator status does not extend to a channel the account has left.
	c.owner.mu.Lock()
	v := c.owner.conversations[conv.ID]
	v.Joined = false
	c.owner.conversations[conv.ID] = v
	c.owner.mu.Unlock()
	if _, err = m.RedactAs(ctx, "alice", conv.ID, "network:m3", ""); !errors.Is(err, ErrDeleteForbidden) {
		t.Fatalf("an operator deleted in a channel they left: %v", err)
	}
}

func TestRedactAsRefusesOthersDirectMessages(t *testing.T) {
	m, c, _, _ := redactFixture(t, map[string]bool{"message-tags": true, "draft/message-redaction": true})
	c.owner.mu.Lock()
	dm := c.conversation("Peer")
	dm.Joined = true
	dm.MemberModes = map[string]string{"alice": "@"}
	c.owner.conversations[dm.ID] = dm
	c.owner.messages[dm.ID] = []Message{{ID: "network:d1", UpstreamID: "d1", ConversationID: dm.ID, SenderID: "peer", Kind: "message", Text: "hi", Reactions: []Reaction{}}}
	c.owner.mu.Unlock()
	if _, err := m.RedactAs(context.Background(), "alice", dm.ID, "network:d1", ""); !errors.Is(err, ErrDeleteForbidden) {
		t.Fatalf("deleted someone else's direct message: %v", err)
	}
}

func TestIncomingRedactIgnoresHalfOperatorsButTrustsTheAssociatedServer(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{"message-tags": true, "draft/message-redaction": true})
	c.owner.mu.Lock()
	half := c.member("Half", "")
	c.owner.mu.Unlock()
	setMemberModes(c, conv, map[string]string{half.ID: "%"})
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "Half", Ident: "h", Host: "h"}, Params: []string{"#test", "m2"}})
	if stored(c, conv.ID, "network:m2").Deleted {
		t.Fatal("a half-operator's REDACT was applied")
	}
	// On the associated server the moderation login deletes without being in
	// the channel; the server has already checked its authority.
	m.options.Home = NetworkConfig{ID: "network", Host: "irc.example.test", Port: 6697, TLS: true}
	c.config.Host, c.config.Port, c.config.TLS = "irc.example.test", 6697, true
	c.redact(girc.Event{Command: "REDACT", Source: &girc.Source{Name: "ircordsvcmod", Ident: "ircord", Host: "h"}, Params: []string{"#test", "m2", "spam"}})
	if !stored(c, conv.ID, "network:m2").Deleted {
		t.Fatal("the associated server's relayed REDACT was ignored")
	}
}

// fakeModerationServer speaks just enough IRC for the moderation login: CAP,
// SASL PLAIN, OPER, and labeled REDACT answered by respond.
type fakeModerationServer struct {
	t       *testing.T
	ln      net.Listener
	mu      sync.Mutex
	lines   []string
	respond func(label string, params []string) string
}

func newFakeModerationServer(t *testing.T, respond func(label string, params []string) string) *fakeModerationServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeModerationServer{t: t, ln: ln, respond: respond}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	return s
}

func (s *fakeModerationServer) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *fakeModerationServer) received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.lines...)
}

func (s *fakeModerationServer) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	send := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	nick := "*"
	for {
		raw, err := r.ReadString('\n')
		if err != nil {
			return
		}
		e := girc.ParseEvent(strings.TrimRight(raw, "\r\n"))
		if e == nil {
			continue
		}
		s.mu.Lock()
		s.lines = append(s.lines, e.Command+" "+strings.Join(e.Params, " "))
		s.mu.Unlock()
		switch e.Command {
		case "CAP":
			if len(e.Params) > 0 && e.Params[0] == "LS" {
				send(":fake CAP * LS :sasl labeled-response message-tags batch draft/message-redaction")
			} else if len(e.Params) > 1 && e.Params[0] == "REQ" {
				send(":fake CAP * ACK :" + e.Last())
			}
		case "NICK":
			nick = e.Params[0]
		case "AUTHENTICATE":
			if e.Params[0] == "PLAIN" {
				send("AUTHENTICATE +")
			} else {
				send(":fake 900 " + nick + " " + nick + "!u@h " + nick + " :You are now logged in")
				send(":fake 903 " + nick + " :SASL authentication successful")
			}
		case "USER":
		case "PING":
			send(":fake PONG fake :" + e.Last())
		case "OPER":
			if e.Params[1] == "right-password" {
				send(":fake 381 " + nick + " :You are now an IRC operator")
			} else {
				send(":fake 464 " + nick + " :Password incorrect")
			}
		case "REDACT":
			send("@label=" + e.Tags["label"] + " " + s.respond(e.Tags["label"], e.Params))
		}
		if e.Command == "CAP" && len(e.Params) > 0 && e.Params[0] == "END" {
			send(":fake 001 " + nick + " :Welcome")
			send(":fake 376 " + nick + " :End of MOTD")
		}
	}
}

// fakeErgoAPI accepts account registration for the moderation account.
func fakeErgoAPI(t *testing.T, ircPort int) *ergoapi.Client {
	t.Helper()
	var mu sync.Mutex
	accounts := map[string]string{}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/v1/ns/saregister":
			if _, ok := accounts[in["accountName"]]; ok {
				_, _ = w.Write([]byte(`{"success":false,"errorCode":"ACCOUNT_EXISTS"}`))
				return
			}
			accounts[in["accountName"]] = in["passphrase"]
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/v1/check_auth":
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": accounts[in["accountName"]] == in["passphrase"]})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)
	client, err := ergoapi.New(ergoapi.Options{Server: "127.0.0.1:" + strconv.Itoa(ircPort), URL: api.URL, Token: id() + id(), IRCPlaintext: true, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func ownerFixture(t *testing.T, operPassword string, respond func(string, []string) string) (*Manager, *storage.Store, *connection, Conversation, *fakeModerationServer) {
	t.Helper()
	server := newFakeModerationServer(t, respond)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []string{"alice", "bob"} {
		if _, err = store.CreateAccount(account, account+"-key-hash"); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.GrantOwner("alice", storage.OwnerFromOwner, "test"); err != nil {
		t.Fatal(err)
	}
	home := NetworkConfig{ID: "network", Host: "127.0.0.1", Port: server.port()}
	m := New(Options{Store: store, Home: home, AssociatedServer: fakeErgoAPI(t, server.port()), Moderation: &ModerationLogin{Operator: "ircord", Password: operPassword}, ConnectTimeout: 10 * time.Second})
	t.Cleanup(func() { m.Close(); store.Close() })
	w := m.space("alice")
	w.mu.Lock()
	defer w.mu.Unlock()
	w.members["peer"] = Member{ID: "peer", Nick: "Peer", Account: "peeracct"}
	c := &connection{owner: w, manager: m, config: home, ctx: m.ctx, ready: true, caps: map[string]bool{"message-tags": true, "draft/message-redaction": true}, nicks: map[string]string{"peer": "peer"}, historyBatches: map[string]bool{}, batchLabels: map[string]string{}, queue: make(chan []*girc.Event, 8)}
	w.connections["network"] = c
	conv := c.conversation("#test")
	conv.Joined = true
	w.conversations[conv.ID] = conv
	w.messages[conv.ID] = []Message{
		{ID: "network:m1", UpstreamID: "m1", ConversationID: conv.ID, SenderID: "peer", SenderAccount: "peeracct", Kind: "message", Text: "abuse", Reactions: []Reaction{}, Edits: []model.MessageEdit{{UpstreamID: "m1e"}}},
		{ID: "network:m2", UpstreamID: "m2", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Text: "more abuse", Reactions: []Reaction{}},
	}
	return m, store, c, conv, server
}

func TestServerOwnerDeletesThroughTheModerationLogin(t *testing.T) {
	m, store, c, conv, server := ownerFixture(t, "right-password", func(_ string, params []string) string {
		if params[1] == "m2" {
			return ":fake FAIL REDACT UNKNOWN_MSGID " + params[0] + " " + params[1] + " :This message does not exist or is too old"
		}
		return ":fake ACK"
	})
	if got := m.ModerationCapability("alice"); got == nil || got.NetworkID != "network" {
		t.Fatalf("owner capability: %+v", got)
	}
	if got := m.ModerationCapability("bob"); got != nil {
		t.Fatalf("a non-owner was offered server-owner deletion: %+v", got)
	}
	r, err := m.RedactAs(context.Background(), "alice", conv.ID, "network:m1", "abusive")
	if err != nil {
		t.Fatal(err)
	}
	if r.Authority != AuthorityOwner || !r.Message.Deleted || !stored(c, conv.ID, "network:m1").Deleted || r.Original.SenderAccount != "peeracct" || r.Original.Text != "" {
		t.Fatalf("result: %+v", r)
	}
	if len(c.queue) != 0 {
		t.Fatal("the owner's own connection sent the deletion")
	}
	lines := strings.Join(server.received(), "\n")
	for _, want := range []string{"OPER ircord right-password", "REDACT #test m1 abusive", "REDACT #test m1e"} {
		if !strings.Contains(lines, want) {
			t.Fatalf("the moderation login did not send %q:\n%s", want, lines)
		}
	}
	r, err = m.RedactAs(context.Background(), "alice", conv.ID, "network:m2", "")
	if !errors.Is(err, ErrRedactRefused) || !tooOld(err) || r.Authority != AuthorityOwner {
		t.Fatalf("refusal: %v %+v", err, r)
	}
	if stored(c, conv.ID, "network:m2").Deleted {
		t.Fatal("a refused deletion changed history")
	}
	// Revoking ownership applies to the next request.
	if err = store.GrantOwner("bob", storage.OwnerFromOwner, "test"); err != nil {
		t.Fatal(err)
	}
	if err = store.RevokeOwner("alice", false); err != nil {
		t.Fatal(err)
	}
	if _, err = m.RedactAs(context.Background(), "alice", conv.ID, "network:m2", ""); !errors.Is(err, ErrDeleteForbidden) {
		t.Fatalf("a revoked owner deleted: %v", err)
	}
}

func TestServerOwnerDeletionReportsARefusedOperatorLogin(t *testing.T) {
	m, _, c, conv, _ := ownerFixture(t, "wrong-password", func(string, []string) string { return ":fake ACK" })
	r, err := m.RedactAs(context.Background(), "alice", conv.ID, "network:m1", "")
	var service *ServiceError
	if !errors.As(err, &service) || !strings.Contains(service.Error(), "refused the moderation operator login") || r.Authority != AuthorityOwner {
		t.Fatalf("got %v %+v", err, r)
	}
	if stored(c, conv.ID, "network:m1").Deleted {
		t.Fatal("deleted without the network's confirmation")
	}
}

func TestModeratorIdentityIsStableAndUnguessable(t *testing.T) {
	a := &moderator{login: ModerationLogin{Operator: "ircord", Password: "one"}}
	b := &moderator{login: ModerationLogin{Operator: "ircord", Password: "two"}}
	accountA, passwordA := a.identity()
	accountA2, _ := a.identity()
	accountB, _ := b.identity()
	if accountA != accountA2 || accountA == accountB || !strings.HasPrefix(accountA, "ircordsvc") || len(accountA) > 21 || strings.Contains(passwordA, "one") {
		t.Fatalf("identities: %q %q %q", accountA, accountA2, accountB)
	}
}

func TestModeratorReadsBatchedLabeledAnswers(t *testing.T) {
	mod := &moderator{waits: map[string]chan string{}, batches: map[string]string{}}
	ok, refused := make(chan string, 1), make(chan string, 1)
	mod.waits["a"], mod.waits["b"] = ok, refused
	mod.receive(girc.Event{Command: "BATCH", Params: []string{"+x", "labeled-response"}, Tags: girc.Tags{"label": "a"}})
	mod.receive(girc.Event{Command: "REDACT", Params: []string{"#c", "m"}, Tags: girc.Tags{"batch": "x"}})
	mod.receive(girc.Event{Command: "BATCH", Params: []string{"-x"}})
	mod.receive(girc.Event{Command: "FAIL", Params: []string{"REDACT", "REDACT_FORBIDDEN", "#c", "m", "You are not authorized"}, Tags: girc.Tags{"label": "b"}})
	if got := <-ok; got != "" {
		t.Fatalf("batched success reported %q", got)
	}
	if got := <-refused; got != "You are not authorized" {
		t.Fatalf("refusal %q", got)
	}
}

func TestRecentMessagesSelectsOnePersonInRange(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{"message-tags": true, "draft/message-redaction": true})
	now := time.Now().UTC()
	c.owner.mu.Lock()
	c.owner.members["peer"] = Member{ID: "peer", Nick: "Peer", Account: "peeracct"}
	c.owner.messages[conv.ID] = []Message{
		{ID: "network:old", UpstreamID: "old", ConversationID: conv.ID, SenderID: "peer", Kind: "message", CreatedAt: now.Add(-2 * time.Hour), Reactions: []Reaction{}},
		{ID: "network:renamed", UpstreamID: "renamed", ConversationID: conv.ID, SenderID: "someone", SenderAccount: "PeerAcct", Kind: "message", CreatedAt: now.Add(-50 * time.Minute), Reactions: []Reaction{}},
		{ID: "network:mine", UpstreamID: "mine", ConversationID: conv.ID, SenderID: "alice", Kind: "message", CreatedAt: now.Add(-40 * time.Minute), Reactions: []Reaction{}},
		{ID: "network:gone", UpstreamID: "gone", ConversationID: conv.ID, SenderID: "peer", Kind: "message", Deleted: true, CreatedAt: now.Add(-30 * time.Minute), Reactions: []Reaction{}},
		{ID: "network:join", ConversationID: conv.ID, SenderID: "peer", Kind: "event", CreatedAt: now.Add(-20 * time.Minute), Reactions: []Reaction{}},
		{ID: "network:new", UpstreamID: "new", ConversationID: conv.ID, SenderID: "peer", Kind: "message", CreatedAt: now.Add(-10 * time.Minute), Reactions: []Reaction{}},
	}
	c.owner.mu.Unlock()
	ctx := context.Background()
	if _, err := m.RecentMessages(ctx, "alice", conv.ID, "peer", time.Hour); !errors.Is(err, ErrDeleteForbidden) {
		t.Fatalf("a member without operator status listed messages to delete: %v", err)
	}
	setMemberModes(c, conv, map[string]string{"alice": "@"})
	if _, err := m.RecentMessages(ctx, "alice", conv.ID, "peer", 8*24*time.Hour); err == nil {
		t.Fatal("accepted a range longer than 7 days")
	}
	got, err := m.RecentMessages(ctx, "alice", conv.ID, "peer", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, msg := range got.Messages {
		ids = append(ids, msg.UpstreamID)
	}
	if strings.Join(ids, ",") != "new,renamed" || got.Nick != "Peer" || got.Authority != AuthorityOperator || got.Channel != "#test" {
		t.Fatalf("selection %v %+v", ids, got)
	}
}

func TestStartPurgeDeletesEachMessageAndReports(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{"message-tags": true, "draft/message-redaction": true, "labeled-response": true})
	now := time.Now().UTC()
	c.owner.mu.Lock()
	c.owner.messages[conv.ID] = []Message{
		{ID: "network:a", UpstreamID: "a", ConversationID: conv.ID, SenderID: "peer", Kind: "message", CreatedAt: now.Add(-3 * time.Minute), Reactions: []Reaction{}},
		{ID: "network:b", UpstreamID: "b", ConversationID: conv.ID, SenderID: "peer", Kind: "message", CreatedAt: now.Add(-2 * time.Minute), Reactions: []Reaction{}},
		{ID: "network:c", UpstreamID: "c", ConversationID: conv.ID, SenderID: "peer", Kind: "message", CreatedAt: now.Add(-time.Minute), Reactions: []Reaction{}},
	}
	c.owner.mu.Unlock()
	setMemberModes(c, conv, map[string]string{"alice": "@"})
	finished := make(chan PurgeJob, 1)
	job, err := m.StartPurge(context.Background(), "alice", conv.ID, "peer", time.Hour, "spam", func(j PurgeJob) { finished <- j })
	if err != nil {
		t.Fatal(err)
	}
	if job.Total != 3 || job.State != "running" || job.Authority != AuthorityOperator {
		t.Fatalf("job %+v", job)
	}
	if _, err = m.StartPurge(context.Background(), "alice", conv.ID, "peer", time.Hour, "", nil); !errors.Is(err, ErrPurgeRunning) {
		t.Fatalf("a second job for the same channel started: %v", err)
	}
	// Answer each REDACT in turn: two accepted, one too old for the server.
	for i := range 3 {
		sent := <-c.queue
		if sent[0].Params[2] != "spam" {
			t.Fatalf("params %v", sent[0].Params)
		}
		reason := ""
		if i == 1 {
			reason = "This message does not exist or is too old"
		}
		c.redactionResult(sent[0].Tags["label"], reason)
	}
	done := <-finished
	if done.State != "done" || done.Deleted != 2 || done.TooOld != 1 || done.Failed != 0 {
		t.Fatalf("finished %+v", done)
	}
	if got, err := m.Purge("alice", job.ID); err != nil || got.Deleted != 2 {
		t.Fatalf("progress %v %+v", err, got)
	}
	if _, err = m.Purge("bob", job.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another account read the job: %v", err)
	}
}
