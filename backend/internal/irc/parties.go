package irc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

const partyMemberLimit = 32
const partyControlOwner = "internal:party-controller"

// partyTransport is deliberately narrow. Durable intent is written before
// Reconcile; it must confirm protection and grants before returning success.
type partyTransport interface {
	Reconcile(context.Context, storage.PartyRecord) error
}
type partyManager struct {
	native          chan partyNativeEvent
	nativeInvites   map[string]partyNativeInvite
	m               *Manager
	store           *storage.Store
	mu              sync.Mutex
	locks           map[string]*sync.Mutex
	lockUsers       map[string]int
	slots           chan struct{}
	transport       partyTransport
	events          chan partyMessageEvent
	protected       map[string]*girc.Client
	control         *connection
	dirty           map[string]bool
	maintenance     map[string]int
	wake            chan struct{}
	nextMaintenance time.Time
}

func newPartyManager(m *Manager) *partyManager {
	if m.options.Store == nil || m.options.Home.ID == "" || m.options.PartyAccounts == nil {
		return nil
	}
	p := &partyManager{m: m, store: m.options.Store, locks: map[string]*sync.Mutex{}, slots: make(chan struct{}, 8), events: make(chan partyMessageEvent, 128)}
	p.transport = &partyIRCController{p: p}
	p.wake = make(chan struct{}, 1)
	p.native = make(chan partyNativeEvent, 64)
	p.nativeInvites = map[string]partyNativeInvite{}
	return p
}
func (p *partyManager) lock(ctx context.Context, id string) (func(), error) {
	select {
	case p.slots <- struct{}{}:
	default:
		return nil, ErrBusy
	}
	p.mu.Lock()
	if p.lockUsers == nil {
		p.lockUsers = map[string]int{}
	}
	p.lockUsers[id]++
	lock := p.locks[id]
	if lock == nil {
		lock = &sync.Mutex{}
		p.locks[id] = lock
	}
	p.mu.Unlock()
	// At most eight workflows can wait or run, independent of account count.
	lock.Lock()
	release := func() {
		lock.Unlock()
		p.mu.Lock()
		p.lockUsers[id]--
		if p.lockUsers[id] == 0 {
			delete(p.lockUsers, id)
			delete(p.locks, id)
		}
		p.mu.Unlock()
		<-p.slots
	}
	if ctx.Err() != nil {
		release()
		return nil, ctx.Err()
	}
	if v, err := p.store.Party(id); err == nil && (v.ConversionPending || v.State == "converted") && ctx.Value(partyConversionKey{}) != true {
		release()
		return nil, serviceError(409, "This group is becoming an ordinary channel. Use channel settings after ownership transfer completes.")
	}
	return release, nil
}
func (m *Manager) PartyCapability(owner string) *model.PartyCapability {
	if m.parties == nil {
		return nil
	}
	if _, e := m.parties.store.PartyCandidate(owner); e != nil {
		return nil
	}
	return &model.PartyCapability{NetworkID: m.options.Home.ID, MaxMembers: partyMemberLimit, MaxNameLength: 100, MaxDescriptionLength: 500}
}
func (m *Manager) PartyCandidates(owner, search string) ([]model.PartyCandidate, error) {
	return m.PartyCandidatesContext(m.ctx, owner, search)
}
func (m *Manager) PartyCandidatesContext(ctx context.Context, owner, search string) ([]model.PartyCandidate, error) {
	if m.PartyCapability(owner) == nil {
		return nil, ErrUnsupported
	}
	if !utf8.ValidString(search) || utf8.RuneCountInString(search) > 100 {
		return nil, fmt.Errorf("search is too long")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	page, err := m.LookupPeople(ctx, owner, model.PeopleQuery{NetworkID: m.options.Home.ID, Purpose: "party", Query: search, Limit: 50})
	if err != nil {
		return nil, err
	}
	if page.Status != "ready" {
		return nil, serviceError(503, "The IRC account directory is unavailable. Try again shortly.")
	}
	network, err := m.Network(owner, m.options.Home.ID)
	if err != nil {
		return nil, err
	}
	items := make([]model.PartyCandidate, 0, len(page.Items))
	for _, person := range page.Items {
		if !person.Eligible || person.ServicesAccount == "" || person.BackendAccountID == owner || person.BackendAccountID == partyControlOwner || strings.HasPrefix(strings.ToLower(person.ServicesAccount), "ircordsvc") || network.Account != "" && model.FoldIdentity(person.ServicesAccount, network.ISupport["CASEMAPPING"]) == model.FoldIdentity(network.Account, network.ISupport["CASEMAPPING"]) {
			continue
		}
		// Keep the principal cache separate so broad candidate results cannot evict
		// the shared names-only directory while a person is typing.
		key := owner + "\x00" + person.Key
		resolved, err := m.partyPeople.lookup(ctx, key, 15*time.Second, func(entry *peopleCacheEntry) error {
			var err error
			entry.candidate, err = m.ResolvePartyCandidate(ctx, person.ServicesAccount, person.BackendAccountID)
			return err
		})
		if err != nil {
			return nil, err
		}
		candidate := resolved.candidate
		if candidate.LocalAccountID() != owner && candidate.LocalAccountID() != partyControlOwner {
			items = append(items, candidate)
		}
	}
	return items, nil
}
func (m *Manager) Parties(owner string) ([]model.Party, error) {
	if m.parties == nil {
		return []model.Party{}, nil
	}
	all, e := m.parties.store.Parties()
	if e != nil {
		return nil, ErrStorage
	}
	out := []model.Party{}
	for _, p := range all {
		if p.HasMember(m.parties.store.PartyPrincipal(owner)) && p.State != "archived" && p.State != "converted" {
			out = append(out, p.Party)
		}
	}
	return out, nil
}
func (m *Manager) Party(owner, id string) (model.Party, error) {
	if m.parties == nil {
		return model.Party{}, ErrNotFound
	}
	p, e := m.parties.store.Party(id)
	if e != nil || !p.HasMember(m.parties.store.PartyPrincipal(owner)) {
		return model.Party{}, ErrNotFound
	}
	return p.Party, nil
}
func partyText(name, description string) error {
	if !utf8.ValidString(name) || !utf8.ValidString(description) || utf8.RuneCountInString(name) > 100 || utf8.RuneCountInString(description) > 500 || !clean(name, 400) || !clean(description, 2000) {
		return fmt.Errorf("use a name up to 100 characters and a description up to 500 characters")
	}
	return nil
}
func partyConversationID(network, target string) string {
	sum := sha256.Sum256([]byte(girc.ToRFC1459(target)))
	return network + "_" + hex.EncodeToString(sum[:12])
}
func (p *partyManager) save(v *storage.PartyRecord) error {
	before := v.Revision
	v.Revision++
	v.UpdatedAt = time.Now().UTC()
	if e := p.store.SaveParty(*v, before); e != nil {
		return ErrStorage
	}
	return nil
}
func (p *partyManager) reconcile(ctx context.Context, v *storage.PartyRecord) error {
	if v.ConversionPending || v.State == "converted" {
		return serviceError(409, "Finish the channel ownership conversion before changing this group.")
	}
	if e := p.migrateIdentities(ctx, v); e != nil {
		return e
	}
	if e := p.revalidateMembers(ctx, v); e != nil {
		return e
	}
	if p.transport == nil {
		return serviceError(503, "Party control is unavailable. Try again after the server reconnects.")
	}
	for _, owner := range v.Revoked {
		if e := p.revoke(owner, *v); e != nil {
			return e
		}
	}
	if len(v.Members) == 0 {
		if e := p.cancelPartyInvitations(*v); e != nil {
			return e
		}
	}
	if e := p.transport.Reconcile(ctx, *v); e != nil {
		return e
	}
	v.Revoked = nil
	if len(v.Members) == 0 {
		v.State = "archived"
		pending, e := p.store.PartyInvitations(v.ID)
		if e != nil {
			return ErrStorage
		}
		for _, i := range pending {
			if i.State == "pending" {
				if e = p.store.SetPartyInvitation(i.Invitee, i.Invitation.ID, "revoked"); e != nil {
					return ErrStorage
				}
				local := p.localOwner(i.Invitee)
				if local == "" {
					continue
				}
				w := p.m.space(local)
				w.mu.Lock()
				w.emit("invitation.removed", model.InvitationRemoval{ID: i.Invitation.ID})
				delete(w.invitations, i.Invitation.ID)
				w.mu.Unlock()
			}
		}
	} else {
		v.State = "active"
	}
	if e := p.materialize(*v); e != nil {
		return e
	}
	if e := p.save(v); e != nil {
		return e
	}
	if c, e := p.m.connection(partyControlOwner, v.NetworkID); e == nil {
		c.mu.Lock()
		client := c.client
		c.mu.Unlock()
		if ctl, ok := p.transport.(*partyIRCController); ok {
			client = ctl.protectedClient(v.ID)
		}
		p.mu.Lock()
		if p.protected == nil {
			p.protected = map[string]*girc.Client{}
		}
		p.protected[v.ID] = client
		p.mu.Unlock()
	}
	return nil
}
func (m *Manager) CreateParty(ctx context.Context, owner, client, name, description string, accounts []string) (model.Party, error) {
	if m.PartyCapability(owner) == nil {
		return model.Party{}, ErrUnsupported
	}
	if !token(client, 100) {
		return model.Party{}, fmt.Errorf("invalid clientId")
	}
	if e := partyText(name, description); e != nil {
		return model.Party{}, e
	}
	if len(accounts) > 31 {
		return model.Party{}, fmt.Errorf("a party can contain at most 32 members")
	}
	p := m.parties
	requestOwner := owner
	creator, e := p.actor(ctx, owner)
	if e != nil {
		return model.Party{}, e
	}
	owner = creator.PrincipalID()
	unlock, e := p.lock(ctx, "create:"+owner)
	if e != nil {
		return model.Party{}, e
	}
	defer unlock()
	accounts = slices.Clone(accounts)
	slices.Sort(accounts)
	accounts = slices.Compact(accounts)
	accounts = slices.DeleteFunc(accounts, func(v string) bool { return v == owner })
	raw, _ := json.Marshal([]any{name, description, accounts})
	sum := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(sum[:])
	v, e := p.store.PartyByRequest(owner, client)
	if errors.Is(e, storage.ErrNotFound) && requestOwner != owner {
		v, e = p.store.PartyByRequest(requestOwner, client)
	}
	if e == nil {
		partyUnlock, lockErr := p.lock(ctx, v.ID)
		if lockErr != nil {
			return model.Party{}, lockErr
		}
		defer partyUnlock()
		v, e = p.store.Party(v.ID)
		if e != nil {
			return model.Party{}, ErrStorage
		}
		if e = p.migrateIdentities(ctx, &v); e != nil {
			return model.Party{}, e
		}
		if !v.HasMember(owner) {
			return model.Party{}, ErrNotFound
		}
		if v.Fingerprint != fingerprint {
			return model.Party{}, ErrConflict
		}
		if v.State == "active" {
			return v.Party, nil
		}
		e = p.reconcile(ctx, &v)
		if e == nil {
			e = p.publish(v)
		}
		return v.Party, e
	}
	if !errors.Is(e, storage.ErrNotFound) {
		return model.Party{}, ErrStorage
	}
	candidates := []model.PartyCandidate{creator}
	resolved := []string{}
	for _, account := range accounts {
		candidate, err := p.candidate(ctx, account)
		if err != nil || candidate.AccountID == partyControlOwner {
			return model.Party{}, fmt.Errorf("choose an available registered IRC account")
		}
		if candidate.PrincipalID() == creator.PrincipalID() {
			continue
		}
		candidates = append(candidates, candidate)
		resolved = append(resolved, candidate.PrincipalID())
	}
	accounts = resolved
	available, e := p.associatedAccounts(ctx, candidates)
	if e != nil {
		return model.Party{}, e
	}
	if available != nil && !available[strings.ToLower(creator.Username)] {
		return model.Party{}, serviceError(403, "Your account is not available on the associated server.")
	}
	for _, candidate := range candidates[1:] {
		if available != nil && !available[strings.ToLower(candidate.Username)] {
			return model.Party{}, fmt.Errorf("choose an available registered IRC account")
		}
	}
	now := time.Now().UTC()
	channel := "#ircord-" + id() + id()
	channel = channel[:40]
	v = storage.PartyRecord{Party: model.Party{ID: id(), ConversationID: partyConversationID(m.options.Home.ID, channel), NetworkID: m.options.Home.ID, Name: name, Description: description, OwnerID: owner, State: "provisioning", Revision: 1, Members: []model.PartyMember{{PartyCandidate: creator, JoinedAt: now}}, CreatedAt: now, UpdatedAt: now}, Channel: channel, Creator: owner, ClientID: client, Fingerprint: fingerprint}
	if creator.ID != "" {
		v.IdentityVersion = 1
	}
	initial := []storage.PartyInviteRecord{}
	for _, account := range accounts {
		i := model.Invitation{ID: id(), NetworkID: v.NetworkID, ChannelKey: "party:" + v.ID, InviterID: owner, InviterNick: creator.DisplayName, CreatedAt: now, Party: &model.PartyInvitation{ID: v.ID, Name: v.Name, Description: v.Description, InviterAccountID: owner, MemberCount: 1}}
		initial = append(initial, storage.PartyInviteRecord{Invitation: i, Invitee: account, State: "pending"})
	}
	if e = p.store.CreateParty(v, initial...); errors.Is(e, storage.ErrPartyInvitationLimit) {
		return model.Party{}, serviceError(429, "One of these accounts has reached its invitation limit.")
	} else if e != nil {
		return model.Party{}, ErrStorage
	}
	partyUnlock, e := p.lock(ctx, v.ID)
	if e != nil {
		return v.Party, e
	}
	defer partyUnlock()
	if e = p.reconcile(ctx, &v); e == nil {
		e = p.publish(v)
	}
	return v.Party, e
}

func (p *partyManager) authorized(ctx context.Context, owner, id string) (storage.PartyRecord, error) {
	v, e := p.store.Party(id)
	if e != nil || !v.HasMember(owner) || v.State == "archived" {
		return v, ErrNotFound
	}
	return v, nil
}
func (m *Manager) UpdateParty(ctx context.Context, owner, id string, name, description *string) (model.Party, error) {
	if m.parties == nil {
		return model.Party{}, ErrNotFound
	}
	p := m.parties
	actor, e := p.actor(ctx, owner)
	if e != nil {
		return model.Party{}, e
	}
	owner = actor.PrincipalID()
	unlock, e := p.lock(ctx, id)
	if e != nil {
		return model.Party{}, e
	}
	defer unlock()
	v, e := p.authorized(ctx, owner, id)
	if e != nil {
		return model.Party{}, e
	}
	if name != nil {
		v.Name = *name
	}
	if description != nil {
		v.Description = *description
	}
	if e = partyText(v.Name, v.Description); e != nil {
		return model.Party{}, e
	}
	if e = p.save(&v); e == nil {
		e = p.publish(v)
	}
	return v.Party, e
}
func (p *partyManager) invite(ctx context.Context, v storage.PartyRecord, owner, account string) (model.Invitation, error) {
	if v.HasMember(account) {
		return model.Invitation{}, ErrConflict
	}
	candidate, e := p.candidate(ctx, account)
	if e != nil {
		return model.Invitation{}, ErrNotFound
	}
	if e = p.accountAvailable(ctx, candidate.Username); e != nil {
		return model.Invitation{}, e
	}
	inviter, e := p.candidate(ctx, owner)
	if e != nil {
		return model.Invitation{}, ErrNotFound
	}
	invitations, e := p.store.PartyInvitations(v.ID)
	if e != nil {
		return model.Invitation{}, ErrStorage
	}
	pending := 0
	for _, i := range invitations {
		if i.Invitee == account && i.State != "revoked" {
			return i.Invitation, nil
		}
		if i.State == "pending" {
			pending++
		}
	}
	if len(v.Members)+pending >= partyMemberLimit {
		return model.Invitation{}, serviceError(409, "This party has reached its participant limit.")
	}
	i := model.Invitation{ID: id(), NetworkID: v.NetworkID, Channel: "", ChannelKey: "party:" + v.ID, InviterID: owner, InviterNick: inviter.DisplayName, CreatedAt: time.Now().UTC(), Party: &model.PartyInvitation{ID: v.ID, Name: v.Name, Description: v.Description, InviterAccountID: owner, MemberCount: len(v.Members)}}
	record, e := p.store.PutPartyInvitation(v.ID, candidate.PrincipalID(), i)
	if e != nil {
		return model.Invitation{}, serviceError(429, "The invitation limit has been reached.")
	}
	if record.State == "pending" && v.State == "active" {
		p.emitInvitation(account, record.Invitation)
	}
	return record.Invitation, nil
}
func (m *Manager) InviteParty(ctx context.Context, owner, id, account string) (model.Invitation, error) {
	if m.parties == nil {
		return model.Invitation{}, ErrNotFound
	}
	p := m.parties
	actor, e := p.actor(ctx, owner)
	if e != nil {
		return model.Invitation{}, e
	}
	owner = actor.PrincipalID()
	candidate, e := p.candidate(ctx, account)
	if e != nil {
		return model.Invitation{}, e
	}
	account = candidate.PrincipalID()
	unlock, e := p.lock(ctx, id)
	if e != nil {
		return model.Invitation{}, e
	}
	defer unlock()
	v, e := p.authorized(ctx, owner, id)
	if e != nil {
		return model.Invitation{}, e
	}
	return p.invite(ctx, v, owner, account)
}
func (p *partyManager) emitInvitation(owner string, v model.Invitation) {
	p.inviteNative(owner, v)
	owner = p.localOwner(owner)
	if owner == "" {
		return
	}
	w := p.m.space(owner)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.invitations[v.ID]; !ok {
		w.emit("invitation.received", v)
		if w.storageErr == nil {
			w.invitations[v.ID] = v
		}
	}
}
func (m *Manager) TransferParty(ctx context.Context, owner, id, account string) (model.Party, error) {
	if m.parties == nil {
		return model.Party{}, ErrNotFound
	}
	p := m.parties
	actor, e := p.actor(ctx, owner)
	if e != nil {
		return model.Party{}, e
	}
	owner = actor.PrincipalID()
	account = p.store.PartyPrincipal(account)
	unlock, e := p.lock(ctx, id)
	if e != nil {
		return model.Party{}, e
	}
	defer unlock()
	v, e := p.authorized(ctx, owner, id)
	if e != nil {
		return model.Party{}, e
	}
	if v.OwnerID != owner {
		return model.Party{}, ErrForbidden
	}
	if !v.HasMember(account) {
		return model.Party{}, ErrNotFound
	}
	v.OwnerID = account
	if e = p.save(&v); e == nil {
		e = p.publish(v)
	}
	return v.Party, e
}
func (m *Manager) LeaveParty(ctx context.Context, owner, id string) error {
	if m.parties != nil {
		actor, e := m.parties.actor(ctx, owner)
		if e != nil {
			return e
		}
		owner = actor.PrincipalID()
		ctx = context.WithValue(ctx, partyActorKey{}, actor)
	}
	if m.parties != nil {
		if current, e := m.parties.store.Party(id); e == nil && !current.HasMember(owner) {
			var former bool
			if e = m.parties.store.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM party_participants WHERE party=? AND account_id=?)", id, owner).Scan(&former); e != nil {
				return ErrStorage
			}
			if former {
				return nil
			}
			return ErrNotFound
		}
	}
	_, e := m.removePartyMember(ctx, owner, id, owner, true)
	return e
}
func (m *Manager) RemovePartyMember(ctx context.Context, owner, id, account string) (model.Party, error) {
	return m.removePartyMember(ctx, owner, id, account, false)
}
func (m *Manager) removePartyMember(ctx context.Context, owner, id, account string, leave bool) (model.Party, error) {
	if m.parties == nil {
		return model.Party{}, ErrNotFound
	}
	p := m.parties
	actor, e := p.actor(ctx, owner)
	if e != nil {
		return model.Party{}, e
	}
	owner = actor.PrincipalID()
	account = p.store.PartyPrincipal(account)
	unlock, e := p.lock(ctx, id)
	if e != nil {
		return model.Party{}, e
	}
	defer unlock()
	v, e := p.authorized(ctx, owner, id)
	if e != nil {
		return model.Party{}, e
	}
	if !leave && (v.OwnerID != owner || account == owner) {
		return model.Party{}, ErrForbidden
	}
	if !v.HasMember(account) {
		return v.Party, nil
	}
	v.Members = slices.DeleteFunc(v.Members, func(x model.PartyMember) bool { return x.PrincipalID() == account })
	if v.OwnerID == account {
		v.OwnerID = ""
		if len(v.Members) > 0 {
			v.OwnerID = v.Members[0].PrincipalID()
		}
	}
	v.State = "recovering"
	v.Revoked = append(v.Revoked, account)
	if e = p.save(&v); e != nil {
		return model.Party{}, e
	}
	if e = p.reconcile(ctx, &v); e == nil {
		e = p.publish(v)
	}
	return v.Party, e
}
func (m *Manager) AcceptInvitation(ctx context.Context, owner, id string) (Conversation, error) {
	if m.parties != nil {
		if i, e := m.parties.store.PartyInvitation(owner, id); e == nil {
			if v, err := m.parties.store.Party(i.Invitation.Party.ID); err != nil || v.State != "converted" {
				return m.acceptPartyInvitation(ctx, owner, i)
			}
		}
	}
	w := m.space(owner)
	w.mu.Lock()
	i, ok := w.invitations[id]
	w.mu.Unlock()
	if !ok || i.Rejected || i.Party != nil {
		return Conversation{}, ErrNotFound
	}
	return m.Join(owner, i.NetworkID, i.Channel)
}
func (m *Manager) acceptPartyInvitation(ctx context.Context, owner string, i storage.PartyInviteRecord) (Conversation, error) {
	p := m.parties
	candidate, e := p.actor(ctx, owner)
	if e != nil {
		return Conversation{}, e
	}
	owner = candidate.PrincipalID()
	unlock, e := p.lock(ctx, i.Invitation.Party.ID)
	if e != nil {
		return Conversation{}, e
	}
	defer unlock()
	i, e = p.store.PartyInvitation(owner, i.Invitation.ID)
	if e != nil {
		return Conversation{}, ErrNotFound
	}
	v, e := p.store.Party(i.Invitation.Party.ID)
	if e != nil || v.State == "archived" || len(v.Members) == 0 {
		return Conversation{}, ErrNotFound
	}
	if v.HasMember(owner) {
		if v.State != "active" {
			if e = p.reconcile(ctx, &v); e != nil {
				return Conversation{}, e
			}
		}
		if e = p.publish(v); e != nil {
			return Conversation{}, e
		}
		if e = p.store.SetPartyInvitation(owner, i.Invitation.ID, "accepted"); e != nil {
			return Conversation{}, ErrStorage
		}
		p.cancelNativeInvite(i.Invitation.ID)
		local := p.localOwner(owner)
		if local == "" {
			return p.conversation(v), nil
		}
		w := m.space(local)
		w.mu.Lock()
		w.emit("invitation.removed", model.InvitationRemoval{ID: i.Invitation.ID})
		delete(w.invitations, i.Invitation.ID)
		w.mu.Unlock()
		return p.conversation(v), nil
	}
	if i.State != "pending" || len(v.Members) >= partyMemberLimit {
		return Conversation{}, ErrNotFound
	}
	if e = p.accountAvailable(ctx, candidate.Username); e != nil {
		return Conversation{}, e
	}
	v.Members = append(v.Members, model.PartyMember{PartyCandidate: candidate, JoinedAt: time.Now().UTC()})
	v.State = "recovering"
	if e = p.save(&v); e != nil {
		return Conversation{}, e
	}
	if e = p.reconcile(ctx, &v); e != nil {
		return Conversation{}, e
	}
	if e = p.store.SetPartyInvitation(owner, i.Invitation.ID, "accepted"); e != nil {
		return Conversation{}, ErrStorage
	}
	p.cancelNativeInvite(i.Invitation.ID)
	local := p.localOwner(owner)
	if local != "" {
		e = p.store.MaterializeParty(local, v)
	}
	if e != nil {
		return Conversation{}, ErrStorage
	}
	if e = p.publish(v); e != nil {
		return Conversation{}, e
	}
	if local == "" {
		return p.conversation(v), nil
	}
	w := m.space(local)
	w.mu.Lock()
	w.emit("invitation.removed", model.InvitationRemoval{ID: i.Invitation.ID})
	delete(w.invitations, i.Invitation.ID)
	w.mu.Unlock()
	return p.conversation(v), nil
}

func (p *partyManager) associatedAccounts(ctx context.Context, candidates []model.PartyCandidate) (map[string]bool, error) {
	if p.m.options.PartyAccounts == nil {
		return nil, nil
	}
	// Only inspect the bounded local candidates. A global account directory is
	// unnecessary for eligibility and can be unsupported or exceed response limits.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	jobs := make(chan int, len(candidates))
	for i := range candidates {
		jobs <- i
	}
	close(jobs)
	exists := make([]bool, len(candidates))
	var workers sync.WaitGroup
	for range min(4, len(candidates)) {
		workers.Go(func() {
			for i := range jobs {
				if ctx.Err() != nil {
					return
				}
				available, err := p.m.options.PartyAccounts.AccountExists(ctx, candidates[i].Username)
				if err != nil {
					cancel()
					return
				}
				exists[i] = available
			}
		})
	}
	workers.Wait()
	if ctx.Err() != nil {
		return nil, serviceError(503, "Party account lookup is unavailable. Try again shortly.")
	}
	out := map[string]bool{}
	for i, candidate := range candidates {
		if exists[i] {
			out[strings.ToLower(candidate.Username)] = true
		}
	}
	return out, nil
}
func (p *partyManager) accountAvailable(ctx context.Context, username string) error {
	if p.m.options.PartyAccounts == nil {
		return nil
	}
	exists, e := p.m.options.PartyAccounts.AccountExists(ctx, username)
	if e != nil {
		return serviceError(503, "Party account lookup is unavailable. Try again shortly.")
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func (p *partyManager) cancelPartyInvitations(v storage.PartyRecord) error {
	pending, e := p.store.PartyInvitations(v.ID)
	if e != nil {
		return ErrStorage
	}
	for _, i := range pending {
		if i.State != "pending" {
			continue
		}
		if e = p.store.SetPartyInvitation(i.Invitee, i.Invitation.ID, "revoked"); e != nil {
			return ErrStorage
		}
		p.cancelNativeInvite(i.Invitation.ID)
		local := p.localOwner(i.Invitee)
		if local == "" {
			continue
		}
		w := p.m.space(local)
		w.mu.Lock()
		w.emit("invitation.removed", model.InvitationRemoval{ID: i.Invitation.ID})
		delete(w.invitations, i.Invitation.ID)
		w.mu.Unlock()
	}
	return nil
}
func (p *partyManager) conversation(v storage.PartyRecord) Conversation {
	party := v.Party
	return Conversation{ID: v.ConversationID, NetworkID: v.NetworkID, Kind: "channel", Target: v.Channel, Name: v.Name, Topic: v.Description, Party: &party, MemberIDs: []string{}, MemberModes: map[string]string{}, Joined: false}
}
func (p *partyManager) publish(v storage.PartyRecord) error {
	if v.State == "converted" {
		return nil
	}
	if current, e := p.store.Party(v.ID); e != nil || current.Revision != v.Revision {
		return ErrConflict
	}
	identities, e := p.store.PartyHistoryIdentities(v.ID)
	if e != nil {
		return ErrStorage
	}
	for _, member := range v.Members {
		identities = append(identities, member.PartyCandidate)
	}
	for _, member := range v.Members {
		local := p.localOwner(member.PrincipalID())
		if local == "" {
			continue
		}
		w := p.m.space(local)
		w.mu.Lock()
		if !w.deleted {
			for _, identity := range identities {
				profileID := identity.PrincipalID()
				if identity.LocalAccountID() != "" && p.store.PartyPrincipal(identity.LocalAccountID()) == identity.PrincipalID() {
					profileID = identity.LocalAccountID()
				}
				profile := w.members[profileID]
				if profile.ID == "" {
					profile = Member{ID: profileID, Nick: identity.Username, DisplayName: identity.DisplayName, AvatarURL: identity.AvatarURL, Account: identity.Username, Color: "lavender", Role: "member", Status: "offline", NameSource: "profile"}
					w.members[profile.ID] = profile
					w.emit("member.updated", profile)
				}
			}
			conv := p.conversation(v)
			if old, ok := w.conversations[conv.ID]; ok {
				conv.UnreadCount, conv.MentionCount, conv.Favorite = old.UnreadCount, old.MentionCount, old.Favorite
			}
			conv = w.refreshUnread(conv)
			if c := w.connections[v.NetworkID]; c != nil {
				c.mu.Lock()
				client, ready := c.client, c.ready
				c.mu.Unlock()
				conv.Joined = v.State == "active" && ready && client != nil && client.LookupChannel(v.Channel) != nil && strings.EqualFold(w.networks[v.NetworkID].Account, member.Username)
			}
			w.conversations[conv.ID] = conv
			w.emit("party.updated", v.Party)
			w.emit("conversation.updated", conv)
		}
		e := w.storageErr
		w.mu.Unlock()
		if e != nil {
			return ErrStorage
		}
	}
	p.joinMembers(v)
	invites, e := p.store.PartyInvitations(v.ID)
	if e != nil {
		return ErrStorage
	}
	for _, i := range invites {
		if i.State == "pending" && v.State == "active" {
			p.emitInvitation(i.Invitee, i.Invitation)
		}
	}
	return nil
}
func (p *partyManager) revoke(owner string, v storage.PartyRecord) error {
	if e := p.store.RevokePartyInvitation(v.ID, owner); e != nil {
		return ErrStorage
	}
	owner = p.localOwner(owner)
	if owner == "" {
		return nil
	}
	if e := p.store.PurgePartyCopies(owner, v.ConversationID); e != nil {
		return ErrStorage
	}
	w := p.m.space(owner)
	w.mu.Lock()
	c := w.connections[v.NetworkID]
	if c != nil {
		c.mu.Lock()
		delete(c.desired, c.foldUnlocked(v.Channel))
		c.closedTargets[c.foldUnlocked(v.Channel)] = true
		c.mu.Unlock()
	}
	delete(w.conversations, v.ConversationID)
	delete(w.messages, v.ConversationID)
	w.events = nil
	w.emit("party.removed", model.PartyRemoval{ID: v.ID, ConversationID: v.ConversationID, Revision: v.Revision})
	w.emit("conversation.removed", model.Removal{ID: v.ConversationID})
	w.mu.Unlock()
	if c != nil {
		_ = c.saveTargets()
		if e := c.enqueue(&girc.Event{Command: "PART", Params: []string{v.Channel, "Left party"}}); e != nil {
			c.closeTransport()
		}
	}
	return nil
}

// managedTarget checks exact durable targets, leaving ordinary #ircord-* channels alone.
func (m *Manager) managedTarget(network, target string) bool {
	if m.options.Store == nil || network != m.options.Home.ID {
		return false
	}
	v, e := m.options.Store.PartyByTarget(target)
	return e == nil && v.State != "converted" || e != nil && !errors.Is(e, storage.ErrNotFound)
}
func (m *Manager) partyAccess(owner, conversation string) error {
	if m.options.Store == nil {
		return nil
	}
	v, e := m.options.Store.PartyByConversation(conversation)
	if errors.Is(e, storage.ErrNotFound) {
		var target string
		err := m.options.Store.DB.QueryRow("SELECT COALESCE(json_extract(payload,'$.target'),'') FROM entities WHERE owner=? AND kind='conversation' AND id=?", owner, conversation).Scan(&target)
		if err != nil || target == "" {
			return nil
		}
		if other, err := m.options.Store.PartyByTarget(target); err == nil && other.State != "converted" {
			return ErrForbidden
		}
		return nil
	}
	if e != nil {
		return ErrStorage
	}
	if v.State == "converted" {
		return nil
	}
	if !v.HasMember(m.options.Store.PartyPrincipal(owner)) {
		return ErrNotFound
	}
	if v.ConversionPending {
		return nil
	}
	if v.State != "active" {
		return serviceError(503, "The party is reconnecting. Try again shortly.")
	}
	if m.parties != nil {
		if _, real := m.parties.transport.(*partyIRCController); real && !m.parties.ready(v.ID) {
			return serviceError(503, "The party is reconnecting. Try again shortly.")
		}
	}
	return nil
}
