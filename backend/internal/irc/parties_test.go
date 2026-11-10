package irc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

type partyFixtureTransport struct {
	fail  bool
	calls int
}

func (f *partyFixtureTransport) Reconcile(_ context.Context, p storage.PartyRecord) error {
	f.calls++
	if f.fail {
		return serviceError(503, "Fixture control is unavailable.")
	}
	if !generatedPartyTarget(p.Channel) {
		return errors.New("invalid fixture channel")
	}
	return nil
}
func partyFixture(t *testing.T) (*Manager, *storage.Store, *partyFixtureTransport) {
	t.Helper()
	store, e := storage.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { store.Close() })
	for _, id := range []string{"alice", "bob", "carol", "outsider"} {
		if _, e = store.CreateAccount(id, id); e != nil {
			t.Fatal(e)
		}
	}
	m := New(Options{Store: store, Home: NetworkConfig{ID: "home", Host: "irc.fixture", Port: 6697, TLS: true}})
	t.Cleanup(m.Close)
	transport := &partyFixtureTransport{}
	m.parties = &partyManager{m: m, store: store, locks: map[string]*sync.Mutex{}, slots: make(chan struct{}, 8), transport: transport, events: make(chan partyMessageEvent, 128)}
	return m, store, transport
}
func createFixtureParty(t *testing.T, m *Manager, accounts ...string) model.Party {
	t.Helper()
	p, e := m.CreateParty(context.Background(), "alice", "request-1", "Friends", "Private group", accounts)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func fixturePartyInvite(t *testing.T, s *storage.Store, party, account string) model.Invitation {
	t.Helper()
	account = s.PartyPrincipal(account)
	all, e := s.PartyInvitations(party)
	if e != nil {
		t.Fatal(e)
	}
	for _, i := range all {
		if i.Invitee == account {
			return i.Invitation
		}
	}
	t.Fatal("missing invitation")
	return model.Invitation{}
}

func TestPartyExplicitAcceptanceHistoryAndRevocation(t *testing.T) {
	m, s, _ := partyFixture(t)
	p := createFixtureParty(t, m, "bob")
	if len(p.Members) != 1 {
		t.Fatal("invitation became membership")
	}
	if e := m.LeaveParty(context.Background(), "outsider", p.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("unrelated account learned private party existence through leave", e)
	}
	if _, e := m.Party("bob", p.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("invitee saw party details", e)
	}
	msg := Message{ID: "home:old", UpstreamID: "old", ConversationID: p.ConversationID, SenderID: "alice", SenderAccount: "alice", Text: "retained text", CreatedAt: time.Now().Add(-time.Hour), Kind: "message", Reactions: []Reaction{{Emoji: "+1", MemberIDs: []string{"alice"}}}, Edits: []model.MessageEdit{{UpstreamID: "edit", Text: "retained text", At: time.Now().Add(-time.Minute)}}, OriginalText: "old text"}
	if e := s.SavePartyMessage(p.ID, msg); e != nil {
		t.Fatal(e)
	}
	if e := s.BindPartyUpload("private-file", p.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := m.HistoryContext(context.Background(), "bob", p.ConversationID, "", 40); !errors.Is(e, ErrNotFound) {
		t.Fatal("invitee read history", e)
	}
	i := fixturePartyInvite(t, s, p.ID, "bob")
	conv, e := m.AcceptInvitation(context.Background(), "bob", i.ID)
	if e != nil {
		t.Fatal(e)
	}
	if conv.Party == nil || conv.Party.ID != p.ID {
		t.Fatal("accept did not return party conversation")
	}
	page, e := m.HistoryContext(context.Background(), "bob", p.ConversationID, "", 40)
	if e != nil || len(page.Items) != 1 || page.Items[0].ID != msg.ID || len(page.Items[0].Edits) != 1 || len(page.Items[0].Reactions) != 1 {
		t.Fatalf("full retained message was not materialized: %+v %v", page, e)
	}
	if _, e = m.AcceptInvitation(context.Background(), "bob", i.ID); e != nil {
		t.Fatal("duplicate accept failed", e)
	}
	if _, e = m.RemovePartyMember(context.Background(), "bob", p.ID, "alice"); !errors.Is(e, ErrForbidden) {
		t.Fatal("member removed owner", e)
	}
	if _, e = m.RemovePartyMember(context.Background(), "alice", p.ID, "bob"); e != nil {
		t.Fatal(e)
	}
	if _, e = m.Message("bob", p.ConversationID, msg.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("removed member read message", e)
	}
	if _, e = s.Message("bob", p.ConversationID, msg.ID); !errors.Is(e, storage.ErrNotFound) {
		t.Fatal("removed member retained server copy", e)
	}
	if _, e = m.AcceptInvitation(context.Background(), "bob", i.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("old invitation re-admitted removed member", e)
	}
}

func TestPartyEchoAndControllerCopyShareIdentity(t *testing.T) {
	for _, controllerFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(controllerFirst), func(t *testing.T) {
			m, s, _ := partyFixture(t)
			party := createFixtureParty(t, m)
			w := m.space("alice")
			input := SendInput{ClientID: "party-echo", ConversationID: party.ConversationID, Kind: "message", Text: "One message"}
			if _, _, err := s.BeginCommand("alice", input); err != nil {
				t.Fatal(err)
			}
			c := &connection{manager: m, owner: w, config: NetworkConfig{ID: "home", Nick: "alice", SASLUser: "alice"}, pending: map[string]pending{input.ClientID: {input: input, messageID: "local-pending-id"}}}
			msg := Message{ID: "home:upstream-echo", UpstreamID: "upstream-echo", ConversationID: party.ConversationID, SenderID: "alice", SenderAccount: "alice", Text: input.Text, Kind: "message", CreatedAt: time.Now().UTC(), Reactions: []Reaction{}}
			if err := s.SavePartyMessage(party.ID, msg); err != nil {
				t.Fatal(err)
			}
			event := partyMessageEvent{party: party.ID, message: msg, sender: w.self}
			if controllerFirst {
				m.parties.deliver(event)
			}
			record, err := s.Party(party.ID)
			if err != nil {
				t.Fatal(err)
			}
			c.message(girc.Event{Command: "PRIVMSG", Source: &girc.Source{Name: "alice"}, Params: []string{record.Channel, input.Text}, Timestamp: msg.CreatedAt, Tags: girc.Tags{"account": "alice", "msgid": msg.UpstreamID, "+ircord/client-id": input.ClientID}})
			if !controllerFirst {
				m.parties.deliver(event)
			}
			command, err := s.Command("alice", input.ClientID)
			if err != nil || command.State != "sent" || command.MessageID != msg.ID {
				t.Fatalf("party echo did not acknowledge canonical identity: %+v %v", command, err)
			}
			page, err := s.History("alice", party.ConversationID, "", 40)
			if err != nil || len(page.Items) != 1 || page.Items[0].ID != msg.ID || page.Items[0].ClientID != input.ClientID {
				t.Fatalf("controller and sender echo duplicated or lost correlation: %+v %v", page, err)
			}
		})
	}
}

func TestPartyQueuedSendRevocationReportsKnownDeliveryState(t *testing.T) {
	for _, sent := range []bool{false, true} {
		t.Run(fmt.Sprint(sent), func(t *testing.T) {
			m, s, _ := partyFixture(t)
			party := createFixtureParty(t, m)
			input := SendInput{ClientID: "queued-party-send", ConversationID: party.ConversationID, Kind: "message", Text: "Queued"}
			if _, _, err := s.BeginCommand("alice", input); err != nil {
				t.Fatal(err)
			}
			c := &connection{manager: m, owner: m.space("alice"), config: m.options.Home, pending: map[string]pending{input.ClientID: {input: input}}}
			c.rejectPartySend([]*girc.Event{{Command: "PRIVMSG", Tags: girc.Tags{"+ircord/client-id": input.ClientID}}}, sent)
			command, err := s.Command("alice", input.ClientID)
			expected := "failed"
			if sent {
				expected = "uncertain"
			}
			if err != nil || command.State != expected || command.Error == "" || len(c.pending) != 0 {
				t.Fatalf("revoked queued send retained misleading delivery state: %+v %v", command, err)
			}
		})
	}
}
func TestPartyDeclinesAreDurableAndConcurrentAcceptsAreIdempotent(t *testing.T) {
	m, s, _ := partyFixture(t)
	p := createFixtureParty(t, m, "bob", "carol")
	i := fixturePartyInvite(t, s, p.ID, "bob")
	if e := m.RejectInvitation("bob", i.ID); e != nil {
		t.Fatal(e)
	}
	again, e := m.InviteParty(context.Background(), "alice", p.ID, "bob")
	if e != nil || again.ID != i.ID {
		t.Fatal("duplicate invitation changed identity", e)
	}
	invites, e := s.PendingPartyInvitations("bob")
	if e != nil || len(invites) != 0 {
		t.Fatal("declined invitation prompted again", e)
	}
	if _, e = m.AcceptInvitation(context.Background(), "bob", i.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("declined invitation accepted", e)
	}
	i = fixturePartyInvite(t, s, p.ID, "carol")
	var wg sync.WaitGroup
	failures := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := m.AcceptInvitation(context.Background(), "carol", i.ID)
			failures <- e
		}()
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	current, e := m.Party("alice", p.ID)
	if e != nil || len(current.Members) != 2 {
		t.Fatal("concurrent acceptance duplicated membership", e)
	}
}
func TestPartyRecoverySuccessionArchiveAndCreateIdempotency(t *testing.T) {
	m, s, transport := partyFixture(t)
	transport.fail = true
	p, e := m.CreateParty(context.Background(), "alice", "request-1", "Friends", "Private group", []string{"bob", "carol"})
	serviceStatus(t, e, 503)
	if p.State != "provisioning" {
		t.Fatal("failed creation did not retain recovery state")
	}
	if len(m.Snapshot("alice").Conversations) != 0 {
		t.Fatal("unprotected party was published")
	}
	transport.fail = false
	p = createFixtureParty(t, m, "bob", "carol")
	again := createFixtureParty(t, m, "carol", "bob")
	if p.ID != again.ID {
		t.Fatal("duplicate creation allocated another channel")
	}
	if _, e = m.CreateParty(context.Background(), "alice", "request-1", "Different", "", nil); !errors.Is(e, ErrConflict) {
		t.Fatal("conflicting create request accepted", e)
	}
	for _, account := range []string{"carol", "bob"} {
		i := fixturePartyInvite(t, s, p.ID, account)
		if _, e = m.AcceptInvitation(context.Background(), account, i.ID); e != nil {
			t.Fatal(e)
		}
	}
	if e = m.LeaveParty(context.Background(), "alice", p.ID); e != nil {
		t.Fatal(e)
	}
	next, e := m.Party("carol", p.ID)
	if e != nil || next.OwnerID != "carol" {
		t.Fatal("owner did not pass to earliest accepted member", e)
	}
	if e = m.DropOwner("carol"); e != nil {
		t.Fatal(e)
	}
	next, e = m.Party("bob", p.ID)
	if e != nil || next.OwnerID != "bob" {
		t.Fatal("account deletion did not transfer ownership", e)
	}
	if e = m.LeaveParty(context.Background(), "bob", p.ID); e != nil {
		t.Fatal(e)
	}
	archived, e := s.Party(p.ID)
	if e != nil || archived.State != "archived" || len(archived.Members) != 0 {
		t.Fatal("empty party not archived", e)
	}
	if _, e = s.PartyByTarget(archived.Channel); e != nil {
		t.Fatal("archive lost reserved target", e)
	}
}
func TestPartyGenericCommandsCannotBypassPolicyAndOrdinaryChannelsRemainUsable(t *testing.T) {
	m, s, _ := partyFixture(t)
	p := createFixtureParty(t, m)
	record, e := s.Party(p.ID)
	if e != nil {
		t.Fatal(e)
	}
	c := &connection{manager: m, owner: m.space("alice"), config: m.options.Home}
	for _, line := range []string{"JOIN " + record.Channel, "PRIVMSG ChanServ :AMODE " + record.Channel + " +o alice", "CS CLEAR " + strings.ToUpper(record.Channel) + " access", "MODE " + record.Channel + " -i", "INVITE outsider " + record.Channel, "PART #ordinary," + record.Channel, "JOIN 0"} {
		event, e := rawEvent(line)
		if e != nil {
			t.Fatal(e)
		}
		if !errors.Is(c.denyPartyCommand(event), ErrForbidden) {
			t.Fatal("raw bypass", line)
		}
	}
	for _, line := range []string{"JOIN #ircord-devclub", "PRIVMSG #ircord-devclub :ordinary message", "CS INFO #ircord-devclub"} {
		event, _ := rawEvent(line)
		if e := c.denyPartyCommand(event); e != nil {
			t.Fatal("ordinary channel blocked", line, e)
		}
	}
	if e = m.validatePartyConfig("alice", NetworkConfig{ID: "alias", Host: m.options.Home.Host, Port: 6697, TLS: true, AutoJoin: []string{record.Channel}}); !errors.Is(e, ErrForbidden) {
		t.Fatal("network alias bypass", e)
	}
	if generatedPartyTarget("#ircord-devclub") {
		t.Fatal("ordinary channel treated as generated target")
	}
}
func TestPartyControllerRepliesAreBoundedAndCorrelated(t *testing.T) {
	c := &connection{}
	r := &partyIRCReply{label: "party:one", batches: map[string]bool{}, done: make(chan struct{})}
	c.partyRequest = r
	c.partyReply(nil, *girc.ParseEvent("@label=party:other :server 324 controller #x +isntR"))
	if len(r.events) != 0 {
		t.Fatal("foreign response consumed")
	}
	c.partyReply(nil, *girc.ParseEvent("@label=party:one :server BATCH +a labeled-response"))
	c.partyReply(nil, *girc.ParseEvent("@batch=a :server 324 controller #x +isntRl 2"))
	c.partyReply(nil, *girc.ParseEvent(":server BATCH -a"))
	select {
	case <-r.done:
	default:
		t.Fatal("labeled batch did not finish")
	}
	if len(r.events) != 1 {
		t.Fatal("unexpected reply count")
	}
	first := girc.New(girc.Config{Nick: "first", User: "test", Server: "fixture"})
	second := girc.New(girc.Config{Nick: "second", User: "test", Server: "fixture"})
	r = &partyIRCReply{label: "party:generation", client: first, batches: map[string]bool{}, done: make(chan struct{})}
	c.client = second
	c.partyRequest = r
	c.partyReply(second, *girc.ParseEvent("@label=party:generation :server ACK"))
	if !errors.Is(r.err, ErrDisconnected) {
		t.Fatal("a new transport satisfied an old protection request")
	}
}

func TestPartyInitialInvitationsCommitAtomicallyAndRetry(t *testing.T) {
	m, s, _ := partyFixture(t)
	candidate, _ := s.PartyCandidate("outsider")
	now := time.Now().UTC()
	for n := range 20 {
		id := fmt.Sprintf("capacity-%02d", n)
		record := storage.PartyRecord{Party: model.Party{ID: id, ConversationID: id, NetworkID: "home", OwnerID: "outsider", State: "active", Revision: 1, Members: []model.PartyMember{{PartyCandidate: candidate, JoinedAt: now}}, CreatedAt: now, UpdatedAt: now}, Channel: "#" + id, Creator: "outsider", ClientID: id}
		i := model.Invitation{ID: id, NetworkID: "home", CreatedAt: now, Party: &model.PartyInvitation{ID: id, MemberCount: 1}}
		if e := s.CreateParty(record, storage.PartyInviteRecord{Invitation: i, Invitee: "carol", State: "pending"}); e != nil {
			t.Fatal(e)
		}
	}
	_, e := m.CreateParty(context.Background(), "alice", "atomic", "Friends", "", []string{"bob", "carol"})
	serviceStatus(t, e, 429)
	if _, e = s.PartyByRequest("alice", "atomic"); !errors.Is(e, storage.ErrNotFound) {
		t.Fatal("partially initialized party committed", e)
	}
	if pending, _ := s.PendingPartyInvitations("bob"); len(pending) != 0 {
		t.Fatal("first invite survived second invite failure")
	}
	if e = s.SetPartyInvitation("carol", "capacity-00", "declined"); e != nil {
		t.Fatal(e)
	}
	p, e := m.CreateParty(context.Background(), "alice", "atomic", "Friends", "", []string{"bob", "carol"})
	if e != nil {
		t.Fatal("same request could not retry", e)
	}
	invitations, _ := s.PartyInvitations(p.ID)
	if len(invitations) != 2 {
		t.Fatal("retry lost initial invitation intent")
	}
}

func TestPartyStaleMaterializationCannotRestoreRemovedAccess(t *testing.T) {
	m, s, _ := partyFixture(t)
	p := createFixtureParty(t, m, "bob")
	i := fixturePartyInvite(t, s, p.ID, "bob")
	if _, e := m.AcceptInvitation(context.Background(), "bob", i.ID); e != nil {
		t.Fatal(e)
	}
	stale, _ := s.Party(p.ID)
	msg := Message{ID: "home:private", UpstreamID: "private", ConversationID: p.ConversationID, SenderID: "alice", SenderAccount: "alice", Text: "private", CreatedAt: time.Now(), Kind: "message", Reactions: []Reaction{}}
	if e := s.SavePartyMessage(p.ID, msg); e != nil {
		t.Fatal(e)
	}
	if _, e := m.RemovePartyMember(context.Background(), "alice", p.ID, "bob"); e != nil {
		t.Fatal(e)
	}
	if e := s.MaterializeParty("bob", stale); e == nil {
		t.Fatal("stale copy restored revoked bytes")
	}
	if e := m.parties.publish(stale); e == nil {
		t.Fatal("stale publication restored removed conversation")
	}
	if _, e := m.SaveMessage("bob", p.ConversationID, msg.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("removed message can be bookmarked", e)
	}
	fresh, e := m.InviteParty(context.Background(), "alice", p.ID, "bob")
	if e != nil || fresh.ID == i.ID {
		t.Fatal("former member did not receive a fresh invitation", e)
	}
	if _, e = m.AcceptInvitation(context.Background(), "bob", fresh.ID); e != nil {
		t.Fatal("former member could not explicitly reaccept", e)
	}
}

func TestPartyArchiveRevokesPendingInvitationsAndQuota(t *testing.T) {
	m, s, _ := partyFixture(t)
	p := createFixtureParty(t, m, "bob")
	i := fixturePartyInvite(t, s, p.ID, "bob")
	if e := m.LeaveParty(context.Background(), "alice", p.ID); e != nil {
		t.Fatal(e)
	}
	if pending, e := s.PendingPartyInvitations("bob"); e != nil || len(pending) != 0 {
		t.Fatal("archive retained pending invitation quota", e)
	}
	if len(m.Snapshot("bob").Invitations) != 0 {
		t.Fatal("archive retained recipient invitation banner")
	}
	if _, e := m.AcceptInvitation(context.Background(), "bob", i.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("archived invitation accepted", e)
	}
}

func TestPartyFailedFinalLeaveCannotBeResurrectedByPendingInvite(t *testing.T) {
	m, s, transport := partyFixture(t)
	p := createFixtureParty(t, m, "bob")
	i := fixturePartyInvite(t, s, p.ID, "bob")
	transport.fail = true
	if e := m.LeaveParty(context.Background(), "alice", p.ID); e == nil {
		t.Fatal("fixture should leave durable recovery intent")
	}
	if _, e := m.AcceptInvitation(context.Background(), "bob", i.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("pending invite resurrected empty party", e)
	}
	if pending, _ := s.PendingPartyInvitations("bob"); len(pending) != 0 {
		t.Fatal("failed archival retained pending quota")
	}
	transport.fail = false
	m.parties.restore()
	record, _ := s.Party(p.ID)
	if record.State != "archived" {
		t.Fatal("durable final leave did not recover")
	}
}

func TestPartyInterruptedAcceptanceDoesNotPretendToDeclineDurableMembership(t *testing.T) {
	m, s, transport := partyFixture(t)
	p := createFixtureParty(t, m, "bob")
	i := fixturePartyInvite(t, s, p.ID, "bob")
	transport.fail = true
	if _, e := m.AcceptInvitation(context.Background(), "bob", i.ID); e == nil {
		t.Fatal("fixture should interrupt acceptance")
	}
	serviceStatus(t, m.RejectInvitation("bob", i.ID), 409)
	transport.fail = false
	if _, e := m.AcceptInvitation(context.Background(), "bob", i.ID); e != nil {
		t.Fatal("explicit acceptance did not recover", e)
	}
}

func TestPartyHistoricalReactionOrderingUsesServerTime(t *testing.T) {
	m, s, _ := partyFixture(t)
	p := createFixtureParty(t, m, "bob")
	i := fixturePartyInvite(t, s, p.ID, "bob")
	if _, e := m.AcceptInvitation(context.Background(), "bob", i.ID); e != nil {
		t.Fatal(e)
	}
	record, _ := s.Party(p.ID)
	msg := Message{ID: "home:parent", UpstreamID: "parent", ConversationID: p.ConversationID, SenderID: "alice", SenderAccount: "alice", Text: "history", CreatedAt: time.Now().Add(-time.Minute), Kind: "message", Reactions: []Reaction{}}
	if e := s.SavePartyMessage(p.ID, msg); e != nil {
		t.Fatal(e)
	}
	c := &connection{manager: m, owner: m.space(partyControlOwner), config: NetworkConfig{ID: "home", Nick: "controller"}, nicks: map[string]string{}, historyBatches: map[string]bool{"audit": true}}
	for _, line := range []string{"@account=bob;batch=audit;time=2026-11-10T12:00:02.000Z;+reply=parent;+draft/unreact=ok :bob!u@h TAGMSG " + record.Channel, "@account=bob;batch=audit;time=2026-11-10T12:00:01.000Z;+reply=parent;+draft/react=ok :bob!u@h TAGMSG " + record.Channel} {
		c.reaction(*girc.ParseEvent(line))
	}
	stored, e := s.PartyMessage(p.ID, msg.ID)
	if e != nil || len(stored.Reactions) != 0 {
		t.Fatal("older react resurrected a newer unreact", e)
	}
}

func TestPartyMaterializationSkipsUnchangedCopies(t *testing.T) {
	m, s, _ := partyFixture(t)
	p := createFixtureParty(t, m)
	record, _ := s.Party(p.ID)
	msg := Message{ID: "home:copy", UpstreamID: "copy", ConversationID: p.ConversationID, SenderID: "alice", SenderAccount: "alice", Text: "unchanged", CreatedAt: time.Now(), Kind: "message", Reactions: []Reaction{}}
	if e := s.SavePartyMessage(p.ID, msg); e != nil {
		t.Fatal(e)
	}
	if e := s.MaterializeParty("alice", record); e != nil {
		t.Fatal(e)
	}
	var before, after int64
	if e := s.DB.QueryRow("SELECT total_changes()").Scan(&before); e != nil {
		t.Fatal(e)
	}
	if e := s.MaterializeParty("alice", record); e != nil {
		t.Fatal(e)
	}
	if e := s.DB.QueryRow("SELECT total_changes()").Scan(&after); e != nil {
		t.Fatal(e)
	}
	if after != before {
		t.Fatalf("unchanged history performed %d writes", after-before)
	}
}
