package irc

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"go.yaml.in/yaml/v3"
	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

// This fixture always creates a new disposable container and database. It
// deliberately ignores IRCORD_ERGO_ADDR and all developer/server credentials.
func isolatedPartyErgo(t *testing.T) (string, int, *ergoapi.Client) {
	t.Helper()
	if os.Getenv("IRCORD_PARTY_INTEGRATION") != "1" {
		t.Skip("set IRCORD_PARTY_INTEGRATION=1 for an isolated party controller fixture")
	}
	return isolatedErgo(t, nil)
}

// isolatedErgo starts a disposable Ergo with the private API. configure can
// change the configuration before it is written, for example to add operators.
func isolatedErgo(t *testing.T, configure func(map[string]any)) (string, int, *ergoapi.Client) {
	t.Helper()
	image := "ghcr.io/ergochat/ergo:v2.19.1@sha256:ef885e44f7fa19101bbbc41baef11dc280dc8107465dccaf6f0860f41b48a682"
	raw, e := exec.Command("docker", "run", "--rm", "--entrypoint", "cat", image, "/ircd-bin/default.yaml").Output()
	if e != nil {
		t.Fatal("read pinned fixture defaults", e)
	}
	var cfg map[string]any
	if e = yaml.Unmarshal(raw, &cfg); e != nil {
		t.Fatal(e)
	}
	set := func(keys []string, value any) {
		m := cfg
		for _, key := range keys[:len(keys)-1] {
			next, ok := m[key].(map[string]any)
			if !ok {
				next = map[string]any{}
				m[key] = next
			}
			m = next
		}
		m[keys[len(keys)-1]] = value
	}
	set([]string{"network", "name"}, "PartyFixture")
	set([]string{"server", "name"}, "party.fixture")
	set([]string{"server", "listeners"}, map[string]any{":6667": map[string]any{}})
	set([]string{"server", "ip-limits", "count"}, false)
	set([]string{"server", "ip-limits", "throttle"}, false)
	set([]string{"fakelag", "enabled"}, false)
	set([]string{"accounts", "registration", "throttling", "enabled"}, false)
	set([]string{"accounts", "multiclient", "enabled"}, true)
	set([]string{"accounts", "multiclient", "always-on"}, "opt-in")
	set([]string{"accounts", "nick-reservation", "force-nick-equals-account"}, false)
	set([]string{"channels", "registration", "max-channels-per-account"}, 100)
	set([]string{"history", "enabled"}, true)
	set([]string{"history", "persistent", "enabled"}, true)
	set([]string{"history", "persistent", "unregistered-channels"}, true)
	set([]string{"history", "retention", "allow-individual-delete"}, true)
	set([]string{"history", "retention", "enable-account-indexing"}, true)
	set([]string{"history", "tagmsg-storage", "whitelist"}, []string{"+draft/react", "+draft/unreact"})
	set([]string{"datastore", "sqlite", "enabled"}, true)
	set([]string{"languages", "path"}, "/ircd-bin/languages")
	token := id() + id()
	set([]string{"api", "enabled"}, true)
	set([]string{"api", "listener"}, ":8089")
	set([]string{"api", "bearer-tokens"}, []string{token})
	cfg["opers"] = map[string]any{}
	if configure != nil {
		configure(cfg)
	}
	dir := t.TempDir()
	raw, e = yaml.Marshal(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "ircd.yaml"), raw, 0600); e != nil {
		t.Fatal(e)
	}
	name := "ircord-party-" + id()[:12]
	out, e := exec.Command("docker", "run", "-d", "--name", name, "--memory", "256m", "--pids-limit", "128", "-p", "127.0.0.1::6667", "-p", "127.0.0.1::8089", "-v", dir+":/ircd", image).CombinedOutput()
	if e != nil {
		t.Fatalf("start isolated fixture: %v %s", e, out)
	}
	t.Cleanup(func() {
		if t.Failed() {
			out, _ := exec.Command("docker", "logs", name).CombinedOutput()
			for _, line := range strings.Split(string(out), "\n") {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "history") || strings.Contains(lower, "sqlite") || strings.Contains(lower, "error") {
					t.Log("isolated server", strings.ReplaceAll(line, token, "[redacted]"))
				}
			}
		}
		_ = exec.Command("docker", "rm", "-f", name).Run()
	})
	port := func(containerPort string) int {
		out, e := exec.Command("docker", "port", name, containerPort).Output()
		if e != nil {
			t.Fatal(e)
		}
		_, p, e := net.SplitHostPort(strings.TrimSpace(string(out)))
		if e != nil {
			t.Fatal(e)
		}
		n, e := strconv.Atoi(p)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	ircPort, apiPort := port("6667"), port("8089")
	server := "127.0.0.1:" + strconv.Itoa(ircPort)
	api, e := ergoapi.New(ergoapi.Options{Server: server, URL: "http://127.0.0.1:" + strconv.Itoa(apiPort), Token: token, IRCPlaintext: true, Timeout: 5 * time.Second})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(api.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for api.Status(ctx).State != "ok" {
		if ctx.Err() != nil {
			t.Fatal("isolated API did not become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "127.0.0.1", ircPort, api
}

func TestPartyErgoControllerLifecycle(t *testing.T) {
	host, port, associated := isolatedPartyErgo(t)
	s, e := storage.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	home := NetworkConfig{ID: "home", Host: host, Port: port, Name: "PartyFixture", TLS: false}
	password := id() + id()
	for _, account := range []string{"alice", "bob", "outsider"} {
		if _, e = s.CreateAccount(account, account); e != nil {
			t.Fatal(e)
		}
		if e = associated.Register(context.Background(), account, password); e != nil {
			t.Fatal(e)
		}
	}
	m := New(Options{Store: s, Home: home, AssociatedServer: associated, AllowPrivate: true, ReconnectMin: 20 * time.Millisecond})
	defer func() { m.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	for _, account := range []string{"alice", "bob", "outsider"} {
		cfg := home
		cfg.Nick = account
		cfg.Auth = &model.AuthInput{Method: "sasl-plain", Account: account, Password: &password}
		if _, e = m.Connect(account, cfg); e != nil {
			t.Fatal(e)
		}
		if e = m.WaitConnected(ctx, account, home.ID); e != nil {
			t.Fatal(e)
		}
	}
	p := reconcileFixtureParty(t, m, ctx, seedFixtureParty(t, m, ctx, "bob"))
	record, e := s.Party(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	t.Log("party controller registered and protected its channel")
	eventually(t, func() bool {
		for _, conv := range m.Snapshot("alice").Conversations {
			if conv.ID == p.ConversationID {
				return conv.Joined
			}
		}
		return false
	})
	if e = m.Send("alice", SendInput{ClientID: "first", ConversationID: p.ConversationID, Text: "Before acceptance"}); e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool { msg, _ := s.PartyLatest(p.ID); return msg.Text == "Before acceptance" })
	invitation := fixturePartyInvite(t, s, p.ID, "bob")
	if _, e = m.AcceptInvitation(ctx, "bob", invitation.ID); e != nil {
		t.Fatal("accept party", e)
	}
	page, e := m.HistoryContext(ctx, "bob", p.ConversationID, "", 40)
	if e != nil || len(page.Items) == 0 {
		t.Fatal("accepted member did not receive pre-acceptance history", e)
	}
	if e = m.Nick("bob", home.ID, "bob-renamed"); e != nil {
		t.Fatal("change disposable member nick", e)
	}
	eventually(t, func() bool { n, _ := m.Network("bob", home.ID); return n.Nick == "bob-renamed" })
	if current, e := m.Party("bob", p.ID); e != nil || !slices.ContainsFunc(current.Members, func(member model.PartyMember) bool { return member.AccountID == "bob" && member.Username == "bob" }) {
		t.Fatal("nick change changed party identity", e)
	}
	oldArchive := Message{ID: "home:old-canonical", UpstreamID: "old-canonical", ConversationID: p.ConversationID, SenderID: "alice", SenderAccount: "alice", Text: "Canonical history older than the server", CreatedAt: time.Now().Add(-10 * 24 * time.Hour), Kind: "message", Reactions: []Reaction{}}
	if e = s.SavePartyMessage(p.ID, oldArchive); e != nil {
		t.Fatal(e)
	}
	m.Close()
	m = New(Options{Store: s, Home: home, AssociatedServer: associated, AllowPrivate: true, ReconnectMin: 20 * time.Millisecond})
	if e = m.Restore(); e != nil {
		t.Fatal("restore disposable member networks", e)
	}
	eventually(t, func() bool { return m.parties.ready(p.ID) })
	page, e = m.HistoryContext(ctx, "bob", p.ConversationID, "", 40)
	if e != nil || !slices.ContainsFunc(page.Items, func(msg Message) bool { return msg.Text == "Before acceptance" }) {
		t.Fatal("restart lost full canonical history", e)
	}
	if !slices.ContainsFunc(page.Items, func(msg Message) bool { return msg.ID == oldArchive.ID && msg.Text == oldArchive.Text }) {
		t.Fatal("controller restart treated server expiry as canonical deletion")
	}
	// Raw transport probing uses only our disposable outsider. Ergo itself must
	// reject admission even when a caller knows the exact managed channel name.
	c, _ := m.connection("outsider", home.ID)
	if e = c.enqueue(&girc.Event{Command: "JOIN", Params: []string{record.Channel}}); e != nil {
		t.Fatal(e)
	}
	time.Sleep(300 * time.Millisecond)
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client.LookupChannel(record.Channel) != nil {
		t.Fatal("Ergo admitted an unaccepted account")
	}
	entries, e := m.ChannelAccess(ctx, partyControlOwner, p.ConversationID)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if entry.Account == "alice" || entry.Account == "bob" {
			if entry.Mode != "v" {
				t.Fatal("ordinary party member has moderation rights")
			}
		}
	}
	if _, e = m.RemovePartyMember(ctx, "alice", p.ID, "bob"); e != nil {
		t.Fatal("remove party member", e)
	}
	if _, e = m.HistoryContext(ctx, "bob", p.ConversationID, "", 40); e == nil {
		t.Fatal("removed member read history")
	}
	raw, e := s.PartyControl()
	if e != nil {
		t.Fatal(e)
	}
	var identity partyControlIdentity
	if json.Unmarshal(raw, &identity) != nil || !identity.Registered {
		t.Fatal("controller recovery identity missing")
	}
	var sealed []byte
	if e = s.DB.QueryRow("SELECT payload FROM party_control WHERE id=1").Scan(&sealed); e != nil || strings.Contains(string(sealed), identity.Password) {
		t.Fatal("controller password was not encrypted", e)
	}
	if e = m.LeaveParty(ctx, "alice", p.ID); e != nil {
		t.Fatal("archive party", e)
	}
	archived, e := s.Party(p.ID)
	if e != nil || archived.State != "archived" {
		t.Fatal("party was not archived", e)
	}
}

func TestPartyErgoNativeInviteJoinMessagePart(t *testing.T) {
	host, port, associated := isolatedPartyErgo(t)
	s, e := storage.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.CreateAccount("alice", "alice"); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	password := id() + id()
	for _, account := range []string{"alice", "PeterPowers"} {
		if e = associated.Register(ctx, account, password); e != nil {
			t.Fatal(e)
		}
	}
	home := NetworkConfig{ID: "home", Host: host, Port: port, Name: "Fixture"}
	m := New(Options{Store: s, Home: home, AssociatedServer: associated, AllowPrivate: true, ReconnectMin: 20 * time.Millisecond})
	defer m.Close()
	cfg := home
	cfg.Nick = "alice"
	cfg.Auth = &model.AuthInput{Method: "sasl-plain", Account: "alice", Password: &password}
	if _, e = m.Connect("alice", cfg); e != nil {
		t.Fatal(e)
	}
	if e = m.WaitConnected(ctx, "alice", home.ID); e != nil {
		t.Fatal(e)
	}
	connectNative := func() (*girc.Client, chan struct{}) {
		client := girc.New(girc.Config{Server: host, Port: port, Nick: "PeterPowers", User: "fixture", Name: "Fixture", SASL: &girc.SASLPlain{User: "PeterPowers", Pass: password}, AllowFlood: true})
		connected := make(chan struct{}, 1)
		client.Handlers.Add(girc.CONNECTED, func(_ *girc.Client, _ girc.Event) {
			select {
			case connected <- struct{}{}:
			default:
			}
		})
		done := make(chan struct{})
		go func() { defer close(done); _ = client.Connect() }()
		select {
		case <-connected:
		case <-ctx.Done():
			client.Close()
			t.Fatal("native SASL connection did not complete")
		}
		return client, done
	}
	native, done := connectNative()
	defer func() { native.Close(); <-done }()
	invitations := make(chan string, 1)
	native.Handlers.Add(girc.INVITE, func(_ *girc.Client, event girc.Event) {
		if len(event.Params) > 1 {
			select {
			case invitations <- event.Params[1]:
			default:
			}
		}
	})
	candidate, e := m.ResolvePartyCandidate(ctx, "PeterPowers", "")
	if e != nil {
		t.Fatal(e)
	}
	party := reconcileFixtureParty(t, m, ctx, seedFixtureParty(t, m, ctx, candidate.ID))
	var channel string
	select {
	case channel = <-invitations:
	case <-ctx.Done():
		t.Fatal("registered native account did not receive ordinary IRC INVITE")
	}
	if native.LookupChannel(channel) != nil || len(party.Members) != 1 {
		t.Fatal("INVITE automatically accepted or joined")
	}
	if _, _, e = s.AccountByUsername("PeterPowers"); e != storage.ErrNotFound {
		t.Fatal("native account was fabricated as a Go login", e)
	}
	native.Cmd.Join(channel)
	eventually(t, func() bool {
		v, _ := s.Party(party.ID)
		return v.State == "active" && len(v.Members) == 2 && native.LookupChannel(channel) != nil
	})
	native.Cmd.Message(channel, "Native group message")
	eventually(t, func() bool { msg, _ := s.PartyLatest(party.ID); return msg.Text == "Native group message" })
	native.Close()
	<-done
	eventually(t, func() bool { v, _ := s.Party(party.ID); return len(v.Members) == 2 })
	native, done = connectNative()
	native.Cmd.Join(channel)
	eventually(t, func() bool { return native.LookupChannel(channel) != nil })
	native.Cmd.Part(channel)
	eventually(t, func() bool {
		v, _ := s.Party(party.ID)
		return v.State == "active" && len(v.Members) == 1 && v.Members[0].ServicesAccount == "alice"
	})
	if _, e = m.Party("alice", party.ID); e != nil {
		t.Fatal("native leave changed the remaining member", e)
	}
	t.Log("ordinary native INVITE, explicit JOIN, message, disconnect/rejoin and PART passed without a local native login")
}

func TestPartyErgoExplicitConversionPreservesChannelHistoryAndStopsController(t *testing.T) {
	host, port, associated := isolatedPartyErgo(t)
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	home := NetworkConfig{ID: "home", Host: host, Port: port, Name: "Disposable conversion"}
	password := id() + id()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	m := New(Options{Store: s, Home: home, AssociatedServer: associated, AllowPrivate: true})
	defer func() { m.Close() }()
	for _, account := range []string{"alice", "bob"} {
		if _, err = s.CreateAccount(account, account); err != nil {
			t.Fatal(err)
		}
		if err = associated.Register(ctx, account, password); err != nil {
			t.Fatal(err)
		}
		cfg := home
		cfg.Nick, cfg.SASLUser, cfg.SASLPassword = account, account, password
		if _, err = m.Connect(account, cfg); err != nil {
			t.Fatal(err)
		}
		if err = m.WaitConnected(ctx, account, home.ID); err != nil {
			t.Fatal(err)
		}
	}
	party := reconcileFixtureParty(t, m, ctx, seedFixtureParty(t, m, ctx, "bob"))
	invite := fixturePartyInvite(t, s, party.ID, "bob")
	if _, err = m.AcceptInvitation(ctx, "bob", invite.ID); err != nil {
		t.Fatal(err)
	}
	v, err := s.Party(party.ID)
	if err != nil {
		t.Fatal(err)
	}
	canonical := Message{ID: "home:canonical-retained", UpstreamID: "canonical-retained", ConversationID: v.ConversationID, SenderID: "alice", SenderAccount: "alice", Text: "Local canonical history survives transfer", CreatedAt: time.Now().Add(-10 * 24 * time.Hour), Kind: "message", Reactions: []Reaction{}}
	if err = s.SavePartyMessage(v.ID, canonical); err != nil {
		t.Fatal(err)
	}
	if err = s.MaterializeParty("alice", v); err != nil {
		t.Fatal(err)
	}
	favorite, read := true, true
	if _, err = m.UpdateConversation("alice", v.ConversationID, &read, &favorite); err != nil {
		t.Fatal(err)
	}
	var readBefore int64
	if err = s.DB.QueryRow("SELECT at_ns FROM conversation_read WHERE owner=? AND conversation=?", "alice", v.ConversationID).Scan(&readBefore); err != nil {
		t.Fatal(err)
	}
	control, err := m.connection(partyControlOwner, home.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeAccess, err := control.channelAccess(ctx, v.Channel)
	if err != nil {
		t.Fatal(err)
	}
	modeSnapshot := func(c *connection) map[byte]string {
		replies, err := c.channelExchange(ctx, &girc.Event{Command: "MODE", Params: []string{v.Channel}}, "324")
		if err != nil {
			t.Fatal(err)
		}
		n, err := m.Network("alice", home.ID)
		if err != nil {
			t.Fatal(err)
		}
		values, ok := channelModeValues(replies, n.ISupport)
		if !ok {
			t.Fatal("mode query did not finish")
		}
		return values
	}
	beforeModes := modeSnapshot(control)
	result, err := m.ConvertParty(ctx, "alice", party.ID)
	if err != nil || result.Status != "converted" {
		t.Fatalf("conversion did not finish: %+v %v", result, err)
	}
	converted, err := s.Party(v.ID)
	if err != nil || converted.State != "converted" || converted.Channel != v.Channel || converted.ConversationID != v.ConversationID {
		t.Fatal("conversion changed identity", err)
	}
	info, err := m.ChannelRegistration(ctx, "alice", v.ConversationID)
	if err != nil || !info.Registered || info.Founder != "alice" {
		t.Fatal("actual owner was not transferred", err)
	}
	afterAccess, err := m.ChannelAccess(ctx, "alice", v.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range beforeAccess {
		if entry.Account == "bob" && !slices.Contains(afterAccess, entry) {
			t.Fatal("accepted grant was changed")
		}
	}
	a, _ := m.connection("alice", home.ID)
	if !equal(modeSnapshot(a), beforeModes) {
		t.Fatal("native privacy settings changed")
	}
	var readAfter int64
	if err = s.DB.QueryRow("SELECT at_ns FROM conversation_read WHERE owner=? AND conversation=?", "alice", v.ConversationID).Scan(&readAfter); err != nil || readBefore != readAfter {
		t.Fatal("conversion changed the read marker", err)
	}
	for _, owner := range []string{"alice", "bob"} {
		if _, err = s.Message(owner, v.ConversationID, canonical.ID); err != nil {
			t.Fatal("canonical history was not preserved", err)
		}
		conv := findConversation(t, m, owner, home.ID, v.Channel)
		if conv.Party != nil || conv.ID != v.ConversationID || owner == "alice" && !conv.Favorite {
			t.Fatal("navigation or favorite was not preserved")
		}
	}
	if _, err = m.connection(partyControlOwner, home.ID); err == nil {
		t.Fatal("last converted party left the controller running")
	}
	// All-converted restore opens user connections only. It must not start a
	// controller or perform another ownership transfer during startup.
	m.Close()
	m = New(Options{Store: s, Home: home, AssociatedServer: associated, AllowPrivate: true})
	if err = m.Restore(); err != nil {
		t.Fatal(err)
	}
	if err = m.WaitConnected(ctx, "alice", home.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = m.connection(partyControlOwner, home.ID); err == nil {
		t.Fatal("restore restarted retired controller")
	}
}
