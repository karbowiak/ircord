package irc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func TestLegacyFeatureReplayUsesCurrentContract(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateAccount("alice", "test-key-hash"); err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{"id": "old", "name": "Old network", "status": "connected", "features": map[string]any{"setname": true, "away": true, "redact": true, "metadata": []string{}}}
	if _, err = store.Commit("alice", "network.updated", legacy); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store})
	defer m.Close()
	replay, _, valid, stop := m.Subscribe("alice", 0)
	defer stop()
	if !valid || len(replay) != 1 {
		t.Fatalf("replay failed: %+v", replay)
	}
	var network map[string]any
	if err = json.Unmarshal(replay[0].Data, &network); err != nil {
		t.Fatal(err)
	}
	features := network["features"].(map[string]any)
	for _, key := range []string{"replies", "reactions", "typing", "readMarkers", "history"} {
		if value, ok := features[key]; !ok || value != false {
			t.Fatalf("legacy %s: %v", key, value)
		}
	}
	if _, ok := features["messageLineBytes"]; !ok {
		t.Fatal("legacy event is missing a required limit field")
	}
}

func TestEffectiveFeaturesAndTagPolicy(t *testing.T) {
	c := &connection{ready: true, caps: map[string]bool{"message-tags": true, "batch": true, "draft/multiline": true, "draft/read-marker": true, "server-time": true, "draft/metadata-3": true}, capValues: map[string]string{"draft/multiline": "max-bytes=9000,max-lines=5"}}
	f := c.features(nil)
	if !f.Replies || !f.Reactions || !f.Typing || !f.Attract || !f.ReadMarkers || !slices.Equal(f.Metadata, metadataKeys) || f.Multiline == nil || f.Multiline.MaxBytes != 4096 || f.Multiline.MaxLines != 5 {
		t.Fatalf("rich network: %+v", f)
	}
	withoutMetadata := &connection{ready: true, caps: map[string]bool{}}
	if got := withoutMetadata.features(nil).Metadata; len(got) != 0 {
		t.Fatalf("metadata keys exposed without capability: %v", got)
	}
	f = c.features(map[string]string{"CLIENTTAGDENY": "*,-reply,-typing"})
	if !f.Replies || f.Reactions || !f.Typing || tagAllowed("*", "ircord/gif") {
		t.Fatalf("tag policy: %+v", f)
	}
	if f.Attract {
		t.Fatalf("attract allowed by a deny-all policy: %+v", f)
	}
	if c.features(map[string]string{"CLIENTTAGDENY": "ircord/attract"}).Attract || !c.features(map[string]string{"CLIENTTAGDENY": "*,-ircord/attract"}).Attract {
		t.Fatal("attract ignores CLIENTTAGDENY")
	}
	if (&connection{ready: true, caps: map[string]bool{}}).features(nil).Attract {
		t.Fatal("attract without message-tags")
	}
	f = c.features(map[string]string{"CLIENTTAGDENY": "draft/unreact"})
	if f.Reactions || !f.Replies {
		t.Fatalf("unreact policy: %+v", f)
	}
	c.capValues["draft/multiline"] = "max-lines=3"
	if c.features(nil).Multiline != nil {
		t.Fatal("enabled multiline without required max-bytes")
	}
	c.capValues["draft/multiline"] = "max-bytes=bad,max-lines=3"
	if c.features(nil).Multiline != nil {
		t.Fatal("enabled malformed multiline")
	}
	c.capValues["draft/multiline"] = "max-bytes=100,max-lines=0"
	if c.features(nil).Multiline != nil {
		t.Fatal("enabled invalid max-lines")
	}
	c.capValues["draft/multiline"] = "max-bytes=50"
	delete(c.caps, "batch")
	if c.features(nil).Multiline != nil {
		t.Fatal("enabled multiline without batch")
	}
	c.ready = false
	f = c.features(nil)
	if f.Replies || f.Away || f.Typing || f.ReadMarkers || f.Multiline != nil || len(f.Metadata) != 0 {
		t.Fatalf("stale disconnected features: %+v", f)
	}
}

func TestMessageEventsRespectWireLimits(t *testing.T) {
	f := model.Features{MessageLineBytes: 40, Multiline: &model.MultilineLimits{MaxBytes: 100, MaxLines: 4}}
	for _, text := range []string{strings.Repeat("界", 18), "one\n\nthree", "\x01ACTION one\ntwo\x01"} {
		events, err := messageEvents(f, "#test", text, girc.Tags{"+reply": "m1"})
		if err != nil {
			t.Fatal(err)
		}
		var assembled string
		for _, event := range events[1 : len(events)-1] {
			if event.LenOpts(false) > f.MessageLineBytes || !utf8.ValidString(event.Last()) {
				t.Fatalf("invalid wire line: %q", event.String())
			}
			if assembled != "" {
				if _, concat := event.Tags["draft/multiline-concat"]; !concat {
					assembled += "\n"
				}
			}
			assembled += event.Last()
		}
		if assembled != text {
			t.Fatalf("corrupted %q into %q", text, assembled)
		}
		if events[0].Tags["+reply"] != "m1" || len(events[1].Tags) > 2 {
			t.Fatal("message tags were not kept on opening BATCH")
		}
	}
	for _, tc := range []struct {
		text   string
		limits *model.MultilineLimits
	}{
		{"a\nb", nil},
		{strings.Repeat("x", 26), nil}, // only 25 body bytes fit
		{"abc\ndef", &model.MultilineLimits{MaxBytes: 6, MaxLines: 4}},
		{"a\nb\nc", &model.MultilineLimits{MaxBytes: 100, MaxLines: 2}},
		{strings.Repeat("界", 18), &model.MultilineLimits{MaxBytes: 100, MaxLines: 2}}, // concat lines also count
	} {
		f.Multiline = tc.limits
		if events, err := messageEvents(f, "#test", tc.text, nil); err == nil || len(events) != 0 {
			t.Fatalf("accepted out-of-bounds %q: %+v", tc.text, events)
		}
	}
	f.Multiline = nil
	if events, err := messageEvents(f, "#test", strings.Repeat("x", 25), nil); err != nil || len(events) != 1 {
		t.Fatalf("exact single-line boundary: %v", err)
	}
}

func TestRejectedMutationsDoNotQueueOrChangeHistory(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{"message-tags": true, "draft/message-redaction": true})
	c.pending = map[string]pending{}
	c.owner.networks[c.config.ID] = Network{ID: c.config.ID, ISupport: map[string]string{"CLIENTTAGDENY": "*"}}
	if _, err := m.ToggleReaction("alice", conv.ID, "network:m2", "👍"); !errors.Is(err, ErrUnsupported) {
		t.Fatal("blocked reaction:", err)
	}
	if err := m.Send("alice", SendInput{ClientID: "blocked", ConversationID: conv.ID, Text: "reply", ReplyToID: "network:m2"}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("blocked reply:", err)
	}
	if err := m.Send("alice", SendInput{ClientID: "multiline", ConversationID: conv.ID, Text: "one\ntwo"}); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unsupported multiline:", err)
	}
	if len(c.pending) != 0 || len(c.queue) != 0 || len(stored(c, conv.ID, "network:m2").Reactions) != 0 {
		t.Fatal("denied mutation had side effects")
	}
	c.owner.networks[c.config.ID] = Network{ID: c.config.ID}
	msg := c.owner.messages[conv.ID][1]
	msg.UpstreamID = "_synthetic"
	c.owner.messages[conv.ID][1] = msg
	if _, err := m.ToggleReaction("alice", conv.ID, msg.ID, "👍"); !errors.Is(err, ErrUnsupported) {
		t.Fatal("synthetic target:", err)
	}
	conv.Joined = false
	c.owner.conversations[conv.ID] = conv
	if err := m.Send("alice", SendInput{ClientID: "left", ConversationID: conv.ID, Text: "hello"}); !errors.Is(err, ErrDisconnected) {
		t.Fatal("send after part:", err)
	}
}

// A real TCP negotiation exercises girc's handler ordering, ACK/NAK and CAP
// updates. Offered capabilities deliberately differ from negotiated ones.
type capabilityPeer struct {
	conn     net.Conn
	mu       sync.Mutex
	commands chan girc.Event
}

func (p *capabilityPeer) send(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _ = fmt.Fprint(p.conn, line+"\r\n")
}
func capabilityServer(t *testing.T, offered, rejected string) (int, <-chan *capabilityPeer) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	peers := make(chan *capabilityPeer, 4)
	var wg sync.WaitGroup
	t.Cleanup(func() { _ = listener.Close(); wg.Wait() })
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				p := &capabilityPeer{conn: conn, commands: make(chan girc.Event, 128)}
				peers <- p
				scan := bufio.NewScanner(conn)
				nick := "tester"
				registered := false
				for scan.Scan() {
					e := girc.ParseEvent(scan.Text())
					if e == nil {
						continue
					}
					if e.Command == "PRIVMSG" || e.Command == "BATCH" {
						select {
						case p.commands <- *e:
						default:
						}
					}
					switch e.Command {
					case "NICK":
						nick = e.Last()
					case "PING":
						p.send("PONG :" + e.Last())
					case "CAP":
						switch e.Params[0] {
						case "LS":
							p.send(":test CAP * LS :" + offered)
						case "REQ":
							var ack, nak []string
							for _, cap := range strings.Fields(e.Last()) {
								if cap == rejected {
									nak = append(nak, cap)
								} else {
									ack = append(ack, cap)
								}
							}
							if len(ack) > 0 {
								p.send(":test CAP " + nick + " ACK :" + strings.Join(ack, " "))
							}
							if len(nak) > 0 {
								p.send(":test CAP " + nick + " NAK :" + strings.Join(nak, " "))
							}
						case "END":
							if registered {
								continue
							}
							registered = true
							p.send(":test 001 " + nick + " :Welcome")
							p.send(":test 005 " + nick + " CLIENTTAGDENY=*,-reply PREFIX=(ov)@+ :supported")
							p.send(":test 422 " + nick + " :No MOTD")
						}
					}
				}
			}()
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, peers
}

func TestNegotiationUpdatesPerNetworkFeatures(t *testing.T) {
	offers := "message-tags batch cap-notify server-time draft/multiline=max-bytes=120,max-lines=3 setname"
	richPort, richPeers := capabilityServer(t, offers, "")
	limitedPort, _ := capabilityServer(t, offers, "draft/multiline")
	m := New(Options{AllowPrivate: true, ReconnectMin: 20 * time.Millisecond})
	t.Cleanup(m.Close)
	for _, cfg := range []NetworkConfig{{ID: "rich", Name: "rich", Port: richPort}, {ID: "limited", Name: "limited", Port: limitedPort}} {
		cfg.Host, cfg.Nick = "127.0.0.1", cfg.ID
		if _, err := m.Connect("alice", cfg); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := m.WaitConnected(ctx, "alice", cfg.ID)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	network := func(id string) Network {
		for _, n := range m.Snapshot("alice").Networks {
			if n.ID == id {
				return n
			}
		}
		return Network{}
	}
	if f := network("rich").Features; f.Multiline == nil || f.Multiline.MaxBytes != 120 || f.Multiline.MaxLines != 3 || !f.Replies || f.Reactions || f.Typing {
		t.Fatalf("negotiated rich: %+v", f)
	}
	if network("limited").Features.Multiline != nil {
		t.Fatal("NAK enabled offered multiline")
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("networks: %+v", m.Snapshot("alice").Networks)
			w := m.space("alice")
			w.mu.Lock()
			defer w.mu.Unlock()
			for _, line := range w.trace["rich"] {
				t.Logf("%s %v", line.Command, line.Params)
			}
		}
	})
	peer := <-richPeers
	peer.send(":test CAP rich DEL :message-tags draft/multiline")
	eventually(t, func() bool { f := network("rich").Features; return !f.Replies && f.Multiline == nil })
	if !network("limited").Features.Replies {
		t.Fatal("CAP loss leaked to second network")
	}
	peer.send(":test CAP rich NEW :message-tags draft/multiline=max-bytes=60,max-lines=2")
	eventually(t, func() bool {
		f := network("rich").Features
		return f.Replies && f.Multiline != nil && f.Multiline.MaxBytes == 60
	})
	peer.send(":test 005 rich CLIENTTAGDENY=*,-typing :supported")
	eventually(t, func() bool { f := network("rich").Features; return !f.Replies && f.Typing })
	peer.send(":test CAP rich ACK :-message-tags")
	eventually(t, func() bool { return !network("rich").Features.Typing })
	c, _ := m.connection("alice", "rich")
	c.mu.Lock()
	oldClient := c.client
	c.mu.Unlock()
	_ = peer.conn.Close()
	<-richPeers
	eventually(t, func() bool {
		f := network("rich").Features
		return f.Multiline != nil && f.Multiline.MaxBytes == 120 && f.Replies && !f.Typing
	})
	c.capabilityEvent(oldClient, girc.Event{Command: "CAP", Params: []string{"rich", "DEL", "message-tags draft/multiline"}})
	c.connected(oldClient)
	if f := network("rich").Features; !f.Replies || f.Multiline == nil {
		t.Fatal("previous connection changed current feature state")
	}
	if err := m.Disconnect("alice", "rich"); err != nil {
		t.Fatal(err)
	}
	if f := network("rich").Features; f.Replies || f.Multiline != nil {
		t.Fatalf("stale disconnect: %+v", f)
	}
}

func TestChannelMutationsFollowCurrentPermissions(t *testing.T) {
	m, c, conv, _ := redactFixture(t, map[string]bool{})
	conv.Modes = "it"
	conv.MemberModes = map[string]string{"alice": "+"}
	c.owner.conversations[conv.ID] = conv
	c.owner.networks[c.config.ID] = Network{ID: c.config.ID, ISupport: map[string]string{"PREFIX": "(ohv)@%+"}}
	for _, line := range []string{"KICK #test Peer", "TOPIC #test :change", "INVITE Peer #test", "MODE #test +b Peer!*@*", "SETNAME :Example", "MARKREAD #test", "CHATHISTORY LATEST #test * 10"} {
		if err := m.Raw("alice", c.config.ID, line); err == nil {
			t.Fatalf("accepted unauthorized mutation: %s", line)
		}
	}
	if len(c.queue) != 0 {
		t.Fatal("denied command reached the network")
	}
	conv.MemberModes["alice"] = "%"
	c.owner.conversations[conv.ID] = conv
	if err := m.Raw("alice", c.config.ID, "MODE #test +v Peer"); err != nil {
		t.Fatal(err)
	}
	if err := m.Raw("alice", c.config.ID, "MODE #test +o Peer"); err == nil {
		t.Fatal("halfop granted operator")
	}
	conv.MemberModes["alice"] = "@"
	c.owner.conversations[conv.ID] = conv
	if err := m.SetTopic("alice", conv.ID, "allowed"); err != nil {
		t.Fatal(err)
	}
	conv.Modes = "m"
	conv.MemberModes["alice"] = ""
	c.owner.conversations[conv.ID] = conv
	if err := m.Send("alice", SendInput{ClientID: "moderated", ConversationID: conv.ID, Text: "no voice"}); err == nil {
		t.Fatal("sent to a moderated channel without voice")
	}
	if _, err := m.ToggleReaction("alice", conv.ID, "network:m2", "👍"); err == nil {
		t.Fatal("reacted in a moderated channel without voice")
	}
	conv.Joined = false
	c.owner.conversations[conv.ID] = conv
	if err := m.SetTopic("alice", conv.ID, "not joined"); !errors.Is(err, ErrDisconnected) {
		t.Fatal(err)
	}
}
