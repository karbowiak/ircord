package irc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func TestChannelCreateValidatesAdvertisedSettingsBeforeJoin(t *testing.T) {
	support := map[string]string{"CHANTYPES": "#&", "CHANNELLEN": "30", "CHANMODES": "beI,k,lf,imnstCR", "TOPICLEN": "40"}
	topic := "A short topic"
	plan, err := planChannel(ChannelCreate{Name: "Friends", Topic: &topic, Flags: "is-nt", ModeParams: map[string]string{"k": "private-value", "l": "5"}}, support)
	if err != nil || plan.target != "#Friends" || !plan.desired['s'] || plan.desired['n'] || !plan.desired['k'] {
		t.Fatal("invalid native plan", err)
	}
	for _, input := range []ChannelCreate{
		{Name: "bad name"}, {Name: strings.Repeat("a", 31)}, {Name: "x", Flags: "o"},
		{Name: "x", Flags: "b"}, {Name: "x", Flags: "+k"},
		{Name: "x", ModeParams: map[string]string{"l": "0"}},
		{Name: "x", ModeParams: map[string]string{"k": "bad\rkey"}},
		{Name: "x", Flags: "-k", ModeParams: map[string]string{"k": "key"}},
	} {
		if _, err := planChannel(input, support); err == nil {
			t.Fatal("accepted invalid channel plan")
		}
	}
}

func TestChannelCreationBlocksMessagesAndInvitesUntilConfirmation(t *testing.T) {
	m := New(Options{})
	defer m.Close()
	w := m.space("alice")
	c := &connection{manager: m, owner: w, ctx: context.Background(), config: NetworkConfig{ID: "net"}, explicitCreating: map[string]bool{"#private": true}}
	w.connections["net"] = c
	w.mu.Lock()
	conv := c.conversation("#private")
	w.mu.Unlock()
	if err := m.Send("alice", SendInput{ClientID: "before-protection", ConversationID: conv.ID, Text: "private text"}); err == nil {
		t.Fatal("message was accepted before privacy confirmation")
	}
	for _, line := range []string{"PRIVMSG #private :private text", "INVITE bob #private", "MODE #private -s", "TOPIC #private :new topic"} {
		e := girc.ParseEvent(line)
		if err := c.validateChannelCommand(e); err == nil {
			t.Fatal("channel mutation was accepted during creation")
		}
	}
}

func TestLegacyConversionReturnsAnActionablePrerequisiteForOfflineOwner(t *testing.T) {
	m, s, _ := partyFixture(t)
	party := createFixtureParty(t, m)
	result, err := m.ConvertParty(context.Background(), "alice", party.ID)
	if err != nil || result.Status != "pending" || result.Prerequisite == "" {
		t.Fatal("missing owner connection was not recoverable", err)
	}
	v, err := s.Party(party.ID)
	if err != nil || v.ConversionPending || v.State != "active" {
		t.Fatal("offline precondition changed the existing party", err)
	}
}

func TestChannelAvailabilityIsGenericAndAssociatedEndpointOnly(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/list" {
			t.Error("unexpected privileged request")
			w.WriteHeader(404)
			return
		}
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"channels":[{"name":"#Hidden","topic":"must remain private","owner":"private-owner","userCount":2,"secret":true,"registered":true}]}`))
	}))
	defer upstream.Close()
	associated, err := ergoapi.New(ergoapi.Options{Server: "irc.fixture:6697", URL: upstream.URL, Token: id() + id()})
	if err != nil {
		t.Fatal(err)
	}
	defer associated.Close()
	home := NetworkConfig{ID: "home", Host: "irc.fixture", Port: 6697, TLS: true}
	m := New(Options{Home: home, PartyAccounts: associated})
	defer m.Close()
	w := m.space("alice")
	w.networks[home.ID] = Network{ID: home.ID, ISupport: map[string]string{"CHANMODES": "beI,k,l,imnst", "CASEMAPPING": "rfc1459"}}
	c := &connection{manager: m, owner: w, config: home, ctx: context.Background(), caps: map[string]bool{}}
	w.connections[home.ID] = c
	hidden, err := m.ChannelAvailability(context.Background(), "alice", home.ID, "hidden")
	if err != nil || hidden.Status != "unavailable" {
		t.Fatal("hidden channel hint", err)
	}
	raw, _ := json.Marshal(hidden)
	if string(raw) != `{"status":"unavailable"}` {
		t.Fatal("availability returned privileged details")
	}
	available, err := m.ChannelAvailability(context.Background(), "alice", home.ID, "new")
	if err != nil || available.Status != "available" {
		t.Fatal("fresh name hint", err)
	}
	c.config.TLS = false
	unknown, err := m.ChannelAvailability(context.Background(), "alice", home.ID, "new")
	if err != nil || unknown.Status != "unknown" || calls.Load() != 2 {
		t.Fatal("mismatched endpoint used privileged lookup", err)
	}
	if _, err := m.ChannelAvailability(context.Background(), "bob", home.ID, "hidden"); err != ErrNotFound {
		t.Fatal("foreign network was accepted", err)
	}
}

func TestErgoCreateChannelOwnershipSettingsConflictPartialAndRace(t *testing.T) {
	host, port := ergo(t)
	suffix, password := id()[:10], id()
	accounts := []string{"cca" + suffix, "ccb" + suffix}
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: s, AllowPrivate: true, ReconnectMin: time.Hour})
	defer func() { m.Close(); s.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for i, account := range accounts {
		registerErgoAccount(t, host, port, account, password)
		owner := []string{"alice", "bob"}[i]
		if _, err = s.CreateAccount(owner, owner); err != nil {
			t.Fatal(err)
		}
		if _, err = m.Connect(owner, NetworkConfig{ID: "net", Host: host, Port: port, Name: "Disposable", Nick: account, SASLUser: account, SASLPassword: password}); err != nil {
			t.Fatal(err)
		}
		if err = m.WaitConnected(ctx, owner, "net"); err != nil {
			t.Fatal(err)
		}
	}
	topic, key := "A disposable private channel", "only-fixture-"+suffix
	created, err := m.CreateChannel(ctx, "alice", "net", ChannelCreate{Name: "#create" + suffix, Topic: &topic, Flags: "isnt", ModeParams: map[string]string{"k": key}})
	if err != nil || !created.Registered || !created.Configured || !created.Conversation.Joined {
		t.Fatalf("create did not finish: %+v %v", created, err)
	}
	if created.Conversation.ModeParams["k"] != "" {
		t.Fatal("channel key entered response")
	}
	for _, mode := range "isknt" {
		if !strings.ContainsRune(created.Conversation.Modes, mode) {
			t.Fatal("missing requested mode", string(mode))
		}
	}
	info, err := m.ChannelRegistration(ctx, "alice", created.Conversation.ID)
	if err != nil || info.Founder != accounts[0] {
		t.Fatal("creator is not founder", err)
	}
	// The second user's existing-name attempt must never change topic or modes.
	otherTopic := "must not replace the existing topic"
	if _, err := m.CreateChannel(ctx, "bob", "net", ChannelCreate{Name: strings.ToUpper(created.Conversation.Target), Topic: &otherTopic, Flags: "-is"}); err == nil {
		t.Fatal("existing registered name was accepted")
	}
	current := findConversation(t, m, "alice", "net", created.Conversation.Target)
	if current.Topic != topic || !strings.Contains(current.Modes, "s") {
		t.Fatal("existing channel was reconfigured")
	}
	// Cancel after the upstream JOIN to prove an uncertain/partial result does
	// not claim configuration or delete a real channel as a rollback.
	c, _ := m.connection("alice", "net")
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	partialTarget := "#partial" + suffix
	partialCtx, stop := context.WithCancel(ctx)
	handler := client.Handlers.Add("JOIN", func(_ *girc.Client, e girc.Event) {
		if len(e.Params) > 0 && e.Params[0] == partialTarget {
			stop()
		}
	})
	partial, err := m.CreateChannel(partialCtx, "alice", "net", ChannelCreate{Name: partialTarget, Flags: "is"})
	client.Handlers.Remove(handler)
	stop()
	if err != nil || partial.Configured || len(partial.Warnings) == 0 || partial.Conversation.ID == "" {
		t.Fatal("partial creation was misreported", err)
	}
	eventually(t, func() bool {
		w := m.space("alice")
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.conversations[c.conversationID(partialTarget)].Joined
	})
	// Two local accounts racing for one ordinary IRC name produce one owner.
	type attempt struct {
		owner  string
		result ChannelCreateResult
		err    error
	}
	start, results := make(chan struct{}), make(chan attempt, 2)
	raceTarget := "#race" + suffix
	for _, owner := range []string{"alice", "bob"} {
		go func(owner string) {
			<-start
			result, err := m.CreateChannel(ctx, owner, "net", ChannelCreate{Name: raceTarget, Flags: "isnt"})
			results <- attempt{owner, result, err}
		}(owner)
	}
	close(start)
	wins := 0
	for range 2 {
		v := <-results
		if v.err == nil && v.result.Registered && v.result.Configured {
			wins++
		} else if v.err == nil {
			t.Fatal("race returned unconfirmed success", v.result.Warnings)
		}
	}
	if wins != 1 {
		t.Fatal("creation race winners", wins)
	}
	// Inspect all public state and traces without printing key values.
	w := m.space("alice")
	w.mu.Lock()
	raw, _ := json.Marshal(struct {
		Trace         any
		Events        any
		Conversations any
	}{w.trace, w.events, w.conversations})
	w.mu.Unlock()
	if strings.Contains(string(raw), key) {
		t.Fatal("channel key entered events or diagnostics")
	}
	if _, err := s.PartyControl(); err != storage.ErrNotFound {
		t.Fatal("new channel provisioned a controller", err)
	}
}

func TestChannelCreateNoChanServUsesNativeOwnership(t *testing.T) {
	// The fixture speaks IRC normally; suppress the supported-services marker
	// to exercise the native fallback without inventing a permanent founder.
	host, port := ergo(t)
	nick := "native" + id()[:10]
	m := New(Options{AllowPrivate: true})
	defer m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := m.Connect("owner", NetworkConfig{ID: "net", Host: host, Port: port, Nick: nick}); err != nil {
		t.Fatal(err)
	}
	if err := m.WaitConnected(ctx, "owner", "net"); err != nil {
		t.Fatal(err)
	}
	c, _ := m.connection("owner", "net")
	c.mu.Lock()
	c.ergoServices = false
	c.mu.Unlock()
	v, err := m.CreateChannel(ctx, "owner", "net", ChannelCreate{Name: "#native" + id()[:10], Flags: "isnt"})
	if err != nil || v.Registered || !v.Configured || len(v.Warnings) == 0 {
		t.Fatal("native ownership fallback", err, v.Warnings)
	}
}

func TestLegacyConversionMaterializesCanonicalHistoryWithoutPurgingLocalRows(t *testing.T) {
	m, s, _ := partyFixture(t)
	party := createFixtureParty(t, m, "bob")
	invite := fixturePartyInvite(t, s, party.ID, "bob")
	if _, err := m.AcceptInvitation(context.Background(), "bob", invite.ID); err != nil {
		t.Fatal(err)
	}
	v, _ := s.Party(party.ID)
	v.ConversionPending = true
	if err := m.parties.save(&v); err != nil {
		t.Fatal(err)
	}
	for i, account := range []string{"alice", "bob"} {
		canonical := model.Message{ID: "canonical", UpstreamID: "upstream", ConversationID: v.ConversationID, SenderID: "alice", Text: "retained canonical", CreatedAt: time.Now(), Kind: "message", Reactions: []model.Reaction{}}
		if i == 0 {
			if err := s.SavePartyMessage(v.ID, canonical); err != nil {
				t.Fatal(err)
			}
		}
		local := canonical
		local.ID, local.UpstreamID, local.Text = "local-only", "other-upstream", "keep local retained message"
		if err := s.SaveHistory(account, local); err != nil {
			t.Fatal(err)
		}
		if err := s.MaterializeParty(account, v); err != nil {
			t.Fatal(err)
		}
	}
	converted, err := s.CompletePartyConversion(v)
	if err != nil {
		t.Fatal(err)
	}
	m.parties.publishConversion(converted)
	for _, account := range []string{"alice", "bob"} {
		for _, message := range []string{"canonical", "local-only"} {
			if _, err := s.Message(account, v.ConversationID, message); err != nil {
				t.Fatal("conversion lost retained history", err)
			}
		}
		for _, conv := range m.Snapshot(account).Conversations {
			if conv.ID == v.ConversationID && (conv.Party != nil || conv.Target != v.Channel) {
				t.Fatal("conversion changed channel identity")
			}
		}
	}
	if m.managedTarget(v.NetworkID, v.Channel) {
		t.Fatal("converted channel retained legacy guard")
	}
	// Ordinary users no longer trigger any old membership reconciliation.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); m.parties.restore() }()
	wg.Wait()
}
