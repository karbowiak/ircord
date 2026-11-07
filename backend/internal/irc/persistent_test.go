package irc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
)

func startPeer(t *testing.T, host string, port int, nick string) *girc.Client {
	t.Helper()
	peer := girc.New(girc.Config{Server: host, Port: port, Nick: nick, User: "test", Name: nick, AllowFlood: true})
	ready := make(chan struct{})
	done := make(chan struct{})
	peer.Handlers.Add(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) { close(ready) })
	go func() { defer close(done); _ = peer.Connect() }()
	t.Cleanup(func() {
		peer.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("peer did not close")
		}
	})
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("peer did not connect")
	}
	return peer
}
func TestErgoDurableRestartBackfillAndReplay(t *testing.T) {
	host, port := ergo(t)
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateAccount("alice", "alice-key-hash"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateAccount("bob", "bob-key-hash"); err != nil {
		t.Fatal(err)
	}
	suffix := id()[:8]
	room := "#persist" + suffix
	peer := startPeer(t, host, port, "peer"+suffix)
	manager := New(Options{Store: store, AllowPrivate: true, ReconnectMin: time.Hour})
	closed := false
	t.Cleanup(func() {
		if !closed {
			manager.Close()
			store.Close()
		}
	})
	if _, err = manager.Connect("alice", NetworkConfig{ID: "net", Name: "Ergo", Host: host, Port: port, Nick: "app" + suffix}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = manager.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	conv, err := manager.Join("alice", "net", room)
	if err != nil {
		t.Fatal(err)
	}
	peer.Cmd.Join(room)
	eventually(t, func() bool { return findConversation(t, manager, "alice", "net", room).Joined })
	eventually(t, func() bool { watermark, e := store.Watermark("alice", conv.ID); return e == nil && !watermark.Gap })
	peer.Cmd.Message(room, "anchor")
	eventually(t, func() bool { watermark, e := store.Watermark("alice", conv.ID); return e == nil && watermark.ID != "" })
	cursor := manager.Snapshot("alice").Cursor
	c, _ := manager.connection("alice", "net")
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	client.Close()
	eventually(t, func() bool { return manager.Snapshot("alice").Networks[0].Status != "connected" })
	for i := 0; i < 650; i++ {
		peer.Cmd.Message(room, fmt.Sprintf("missed-%03d", i))
	}
	pong := make(chan struct{})
	peer.Handlers.Add("PONG", func(_ *girc.Client, e girc.Event) {
		if e.Last() == "persist-barrier" {
			select {
			case <-pong:
			default:
				close(pong)
			}
		}
	})
	peer.Send(&girc.Event{Command: "PING", Params: []string{"persist-barrier"}})
	select {
	case <-pong:
	case <-time.After(10 * time.Second):
		t.Fatal("upstream did not finish receiving offline messages")
	}
	manager.Close()
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	closed = true
	store, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager = New(Options{Store: store, AllowPrivate: true})
	defer manager.Close()
	if err = manager.Restore(); err != nil {
		t.Fatal(err)
	}
	// Recovery includes reconnecting and can retry a 15-second IRC request.
	// Use a deadline longer than that request and report state on failure.
	waitRecovered := func(minimum int) {
		t.Helper()
		deadline := time.Now().Add(40 * time.Second)
		n := 0
		for time.Now().Before(deadline) {
			_ = store.DB.QueryRow("SELECT count(*) FROM messages WHERE owner='alice' AND conversation=? AND text LIKE 'missed-%'", conv.ID).Scan(&n)
			if n >= minimum {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		state := manager.Snapshot("alice")
		watermark, _ := store.Watermark("alice", conv.ID)
		w := manager.space("alice")
		w.mu.Lock()
		trace := clone(w.trace["net"])
		w.mu.Unlock()
		if len(trace) > 25 {
			trace = trace[len(trace)-25:]
		}
		t.Fatalf("history recovery: got %d, want at least %d; networks=%+v watermark=%+v trace=%+v", n, minimum, state.Networks, watermark, trace)
	}
	// Interrupt recovery after at least the first page has been imported.
	waitRecovered(100)
	manager.Close()
	manager = New(Options{Store: store, AllowPrivate: true})
	defer manager.Close()
	if err = manager.Restore(); err != nil {
		t.Fatal(err)
	}
	waitRecovered(650)
	eventually(t, func() bool { watermark, e := store.Watermark("alice", conv.ID); return e == nil && !watermark.Gap })
	replay, live, valid, stop := manager.Subscribe("alice", cursor)
	defer stop()
	if !valid || len(replay) == 0 {
		t.Fatal("restart lost the account event cursor")
	}
	for _, event := range replay {
		if event.Seq <= cursor {
			t.Fatal("replayed an already-applied event")
		}
	}
	_ = live
	if _, _, valid, stop := manager.Subscribe("bob", cursor); valid {
		stop()
		t.Fatal("another account accepted Alice's cursor")
	} else {
		stop()
	}
	seen := map[string]bool{}
	before := ""
	missed := 0
	for pages := 0; pages < 50; pages++ {
		page, e := manager.HistoryContext(context.Background(), "alice", conv.ID, before, 40)
		if e != nil {
			t.Fatal(e)
		}
		for _, msg := range page.Items {
			if seen[msg.ID] {
				t.Fatal("duplicate cursor page")
			}
			seen[msg.ID] = true
			if strings.HasPrefix(msg.Text, "missed-") {
				missed++
			}
		}
		if page.NextCursor == nil {
			break
		}
		before = *page.NextCursor
	}
	if missed != 650 {
		t.Fatalf("backfill exposed %d/650 missed messages", missed)
	}
	if _, err = manager.HistoryContext(context.Background(), "bob", conv.ID, "", 40); err != ErrNotFound {
		t.Fatalf("cross-account history accepted: %v", err)
	}
	// Retry idempotency survives process-local cache loss and restart.
	input := SendInput{ClientID: "persistent-command", ConversationID: conv.ID, Text: "stored send"}
	if err = manager.Send("alice", input); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { v, e := store.Command("alice", input.ClientID); return e == nil && v.State == "sent" })
	if err = manager.Send("alice", input); err != nil {
		t.Fatal(err)
	}
	anchor, err := store.Search("alice", "anchor", conv.ID, 1)
	if err != nil || len(anchor) != 1 {
		t.Fatal("missing old reply/reaction target", err)
	}
	if _, err = manager.ToggleReaction("alice", conv.ID, anchor[0].ID, "👍"); err != nil {
		t.Fatal(err)
	}
	if msg, e := manager.ToggleReaction("alice", conv.ID, anchor[0].ID, "👍"); e != nil || len(msg.Reactions) != 0 {
		t.Fatal("old-message unreact failed", e)
	}
	reply := SendInput{ClientID: "reply-to-old", ConversationID: conv.ID, Text: "reply beyond the memory window", ReplyToID: anchor[0].ID}
	if err = manager.Send("alice", reply); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		command, e := store.Command("alice", reply.ClientID)
		if e != nil || command.State != "sent" {
			return false
		}
		msg, e := store.Message("alice", conv.ID, command.MessageID)
		return e == nil && msg.ReplyTo != nil && msg.ReplyTo.ID == anchor[0].ID
	})
	var n int
	if err = store.DB.QueryRow("SELECT count(*) FROM messages WHERE owner='alice' AND conversation=? AND text='stored send'", conv.ID).Scan(&n); err != nil || n != 1 {
		t.Fatal("durable command duplicated", n, err)
	}
}

func TestErgoOfflineDirectDiscovery(t *testing.T) {
	host, port := ergo(t)
	nick := "offline" + id()[:7]
	password := id()
	registered := make(chan struct{}, 1)
	persistent := make(chan struct{}, 1)
	peer := startPeer(t, host, port, nick)
	peer.Handlers.Add("NOTICE", func(_ *girc.Client, e girc.Event) {
		if strings.Contains(e.Last(), "Account created") {
			select {
			case registered <- struct{}{}:
			default:
			}
		}
	})
	peer.Handlers.Add("PERSISTENCE", func(_ *girc.Client, e girc.Event) {
		if e.Last() == "ON" {
			select {
			case persistent <- struct{}{}:
			default:
			}
		}
	})
	peer.Send(&girc.Event{Command: "PRIVMSG", Params: []string{"NickServ", "REGISTER " + password}, Sensitive: true})
	select {
	case <-registered:
	case <-time.After(10 * time.Second):
		t.Fatal("registration failed")
	}
	peer.Send(&girc.Event{Command: "PERSISTENCE", Params: []string{"SET", "ON"}})
	select {
	case <-persistent:
	case <-time.After(10 * time.Second):
		t.Fatal("always-on opt-in failed")
	}
	peer.Close()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateAccount("alice", "key"); err != nil {
		t.Fatal(err)
	}
	manager := New(Options{Store: store, AllowPrivate: true})
	cfg := NetworkConfig{ID: "dm", Name: "DM", Host: host, Port: port, Nick: nick, SASLUser: nick, SASLPassword: password}
	if _, err = manager.Connect("alice", cfg); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = manager.WaitConnected(ctx, "alice", "dm"); err != nil {
		t.Fatal(err)
	}
	manager.Close()
	sender := startPeer(t, host, port, "sender"+id()[:7])
	sender.Cmd.Message(nick, "arrived while IRCord was offline")
	barrier := make(chan struct{}, 1)
	sender.Handlers.Add("PONG", func(_ *girc.Client, e girc.Event) {
		if e.Last() == "dm-barrier" {
			barrier <- struct{}{}
		}
	})
	sender.Send(&girc.Event{Command: "PING", Params: []string{"dm-barrier"}})
	select {
	case <-barrier:
	case <-time.After(5 * time.Second):
		t.Fatal("sender stalled")
	}
	manager = New(Options{Store: store, AllowPrivate: true})
	defer manager.Close()
	if err = manager.Restore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if t.Failed() {
			p, _ := manager.ServerLog("alice", "dm", "", 100, true)
			for _, line := range p.Items {
				if line.Command == "CHATHISTORY" || line.Command == "BATCH" || line.Command == "FAIL" {
					t.Logf("%s %v", line.Command, line.Params)
				}
			}
		}
	})
	eventually(t, func() bool {
		for _, conv := range manager.Snapshot("alice").Conversations {
			if conv.Kind == "direct" {
				page, e := store.History("alice", conv.ID, "", 100)
				if e == nil {
					for _, msg := range page.Items {
						if msg.Text == "arrived while IRCord was offline" {
							return true
						}
					}
				}
			}
		}
		return false
	})
	// A second IRC client on the same upstream account can move the read marker.
	marker := girc.New(girc.Config{Server: host, Port: port, Nick: nick, User: "test", Name: "Marker test", AllowFlood: true, SASL: &girc.SASLPlain{User: nick, Pass: password}, SupportedCaps: map[string][]string{"draft/read-marker": nil}})
	ready := make(chan struct{})
	done := make(chan struct{})
	marker.Handlers.Add(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) { close(ready) })
	go func() { defer close(done); _ = marker.Connect() }()
	defer func() { marker.Close(); <-done }()
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("second account client did not connect")
	}
	var direct Conversation
	for _, conv := range manager.Snapshot("alice").Conversations {
		if conv.Kind == "direct" {
			direct = conv
			break
		}
	}
	if direct.ID == "" {
		t.Fatal("direct conversation missing")
	}
	marker.Send(&girc.Event{Command: "MARKREAD", Params: []string{sender.GetNick(), "timestamp=" + time.Now().UTC().Format(ircTimeLayout)}})
	eventually(t, func() bool { n, e := store.Unread("alice", direct.ID); return e == nil && n == 0 })
}

func TestErgoOlderPagesAndUncertainCommandRecovery(t *testing.T) {
	host, port := ergo(t)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateAccount("alice", "key"); err != nil {
		t.Fatal(err)
	}
	suffix := id()[:7]
	room := "#older" + suffix
	peer := startPeer(t, host, port, "older"+suffix)
	peer.Cmd.Join(room)
	// Complete all writes before the app joins: only its latest window is cached.
	for i := 0; i < 260; i++ {
		peer.Cmd.Message(room, fmt.Sprintf("old-%03d", i))
	}
	barrier := make(chan struct{}, 1)
	peer.Handlers.Add("PONG", func(_ *girc.Client, e girc.Event) {
		if e.Last() == "old-barrier" {
			barrier <- struct{}{}
		}
	})
	peer.Send(&girc.Event{Command: "PING", Params: []string{"old-barrier"}})
	select {
	case <-barrier:
	case <-time.After(10 * time.Second):
		t.Fatal("history sender stalled")
	}
	manager := New(Options{Store: store, AllowPrivate: true})
	defer manager.Close()
	if _, err = manager.Connect("alice", NetworkConfig{ID: "net", Name: "Net", Host: host, Port: port, Nick: "page" + suffix}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = manager.WaitConnected(ctx, "alice", "net"); err != nil {
		t.Fatal(err)
	}
	conv, err := manager.Join("alice", "net", room)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { mark, e := store.Watermark("alice", conv.ID); return e == nil && !mark.Gap })
	cursor := ""
	seen := map[string]bool{}
	count := 0
	for n := 0; n < 20; n++ {
		page, e := manager.HistoryContext(ctx, "alice", conv.ID, cursor, 40)
		if e != nil {
			t.Fatal(e)
		}
		for _, msg := range page.Items {
			if seen[msg.ID] {
				t.Fatal("duplicate history page")
			}
			seen[msg.ID] = true
			if strings.HasPrefix(msg.Text, "old-") {
				count++
			}
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if count != 260 {
		missing := []string{}
		all, _ := store.Search("alice", "old-", conv.ID, 1000)
		known := map[string]bool{}
		for _, msg := range all {
			known[msg.Text] = true
		}
		for i := 0; i < 260; i++ {
			text := fmt.Sprintf("old-%03d", i)
			if !known[text] {
				missing = append(missing, text)
			}
		}
		t.Fatalf("older upstream paging recovered %d/260 (missing in DB %v)", count, missing)
	}
	if results, e := manager.Search("alice", "old-000", conv.ID); e != nil || len(results) != 1 {
		t.Fatal("search omitted old durable messages", e)
	}
	input := SendInput{ClientID: "uncertain", ConversationID: conv.ID, Text: "accepted before crash", Kind: "message"}
	if _, _, err = store.BeginCommand("alice", input); err != nil {
		t.Fatal(err)
	}
	if err = store.UncertainCommand("alice", input.ClientID, "crash"); err != nil {
		t.Fatal(err)
	}
	c, _ := manager.connection("alice", "net")
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	// This emulates an accepted upstream send whose process-local acknowledgment
	// was lost; replay must reconcile it rather than issue another PRIVMSG.
	client.Send(&girc.Event{Command: "PRIVMSG", Params: []string{room, input.Text}, Tags: girc.Tags{"+ircord/client-id": input.ClientID}})
	eventually(t, func() bool {
		command, e := store.Command("alice", input.ClientID)
		return e == nil && command.State == "sent"
	})
	if err = manager.Send("alice", input); err != nil {
		t.Fatal(err)
	}
	var n int
	store.DB.QueryRow("SELECT count(*) FROM messages WHERE owner='alice' AND text=?", input.Text).Scan(&n)
	if n != 1 {
		t.Fatal("uncertain command duplicated")
	}
	// Simulate the durable state immediately before the upstream echo commits.
	// The server retains the accepted message, while the local command is pending.
	previous, err := store.Watermark("alice", conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	crashInput := SendInput{ClientID: "recover-from-history", ConversationID: conv.ID, Text: "upstream accepted before durable acknowledgment", Kind: "message"}
	if err = manager.Send("alice", crashInput); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		command, e := store.Command("alice", crashInput.ClientID)
		return e == nil && command.State == "sent"
	})
	manager.Close()
	if _, err = store.DB.Exec("DELETE FROM messages WHERE owner='alice' AND id=(SELECT message_id FROM outbox WHERE owner='alice' AND client_id=?)", crashInput.ClientID); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB.Exec("UPDATE outbox SET state='pending',message_id=NULL WHERE owner='alice' AND client_id=?", crashInput.ClientID); err != nil {
		t.Fatal(err)
	}
	if err = store.BeginGap("alice", conv.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.AdvanceGap("alice", conv.ID, previous.ID, previous.At); err != nil {
		t.Fatal(err)
	}
	manager = New(Options{Store: store, AllowPrivate: true})
	defer manager.Close()
	if err = manager.Restore(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		command, e := store.Command("alice", crashInput.ClientID)
		return e == nil && command.State == "sent"
	})
	if err = manager.Send("alice", crashInput); err != nil {
		t.Fatal(err)
	}
	store.DB.QueryRow("SELECT count(*) FROM messages WHERE owner='alice' AND text=?", crashInput.Text).Scan(&n)
	if n != 1 {
		t.Fatal("history reconciliation resent an accepted command")
	}
}

func TestErgoPersistentConcurrentOwners(t *testing.T) {
	host, port := ergo(t)
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := New(Options{Store: store, AllowPrivate: true})
	defer manager.Close()
	suffix := id()[:5]
	const owners = 100
	for i := 0; i < owners; i++ {
		owner := fmt.Sprintf("load-%03d", i)
		if _, err = store.CreateAccount(owner, owner); err != nil {
			t.Fatal(err)
		}
		for network := 0; network < 2; network++ {
			if _, err = manager.Connect(owner, NetworkConfig{ID: fmt.Sprint(network), Name: "Ergo", Host: host, Port: port, Nick: fmt.Sprintf("p%s%d%03d", suffix, network, i)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	deadline := time.Now().Add(60 * time.Second)
	connected := 0
	for time.Now().Before(deadline) {
		connected = 0
		for i := 0; i < owners; i++ {
			for _, network := range manager.Snapshot(fmt.Sprintf("load-%03d", i)).Networks {
				if network.Status == "connected" {
					connected++
				}
			}
		}
		if connected == owners*2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if connected != owners*2 {
		t.Fatalf("persistent multi-user connections: %d/%d", connected, owners*2)
	}
	// Verify one owner's durable cursor and conversation are still isolated while
	// all 200 upstream clients are in flight.
	owner := "load-000"
	conv, err := manager.Join(owner, "0", "#private"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		for _, v := range manager.Snapshot(owner).Conversations {
			if v.ID == conv.ID {
				return v.Joined
			}
		}
		return false
	})
	if err = manager.Send(owner, SendInput{ClientID: "load-send", ConversationID: conv.ID, Text: "isolated under concurrent load"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		command, e := store.Command(owner, "load-send")
		return e == nil && command.State == "sent"
	})
	if _, err = manager.HistoryContext(context.Background(), "load-001", conv.ID, "", 40); err != ErrNotFound {
		t.Fatal("load leaked a conversation")
	}
}

func TestErgoLargeAutoJoinAndPerform(t *testing.T) {
	host, port := ergo(t)
	manager := New(Options{AllowPrivate: true})
	defer manager.Close()
	suffix := id()[:6]
	channels := []string{}
	for i := 0; i < 70; i++ {
		channels = append(channels, fmt.Sprintf("#auto%s%02d", suffix, i))
	}
	if _, err := manager.Connect("owner", NetworkConfig{ID: "many", Name: "Many", Host: host, Port: port, Nick: "auto" + suffix, AutoJoin: channels, Perform: []string{"AWAY :joined all configured channels"}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	joined := 0
	for time.Now().Before(deadline) {
		snapshot := manager.Snapshot("owner")
		joined = 0
		for _, conv := range snapshot.Conversations {
			if conv.Joined {
				joined++
			}
		}
		if joined == len(channels) && snapshot.User.Status == "away" {
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatalf("setup commands lost to queue bounds: joined %d/%d", joined, len(channels))
}
