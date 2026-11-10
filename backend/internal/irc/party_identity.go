package irc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

// ResolvePartyCandidate verifies an exact services account and its incarnation.
// backendAccountID is only a hint: it is independently checked against storage.
// Lookup results grant no mutation permission; actors are checked separately.
func (m *Manager) ResolvePartyCandidate(ctx context.Context, servicesAccount, backendAccountID string) (model.PartyCandidate, error) {
	var out model.PartyCandidate
	if m.parties == nil || m.options.PartyAccounts == nil {
		return out, ErrUnsupported
	}
	if !clean(servicesAccount, 128) || strings.TrimSpace(servicesAccount) != servicesAccount || servicesAccount == "" {
		return out, fmt.Errorf("choose a registered IRC account")
	}
	a, err := m.options.PartyAccounts.Account(ctx, servicesAccount)
	if err != nil {
		if err == ergoapi.ErrNotFound {
			return out, ErrNotFound
		}
		return out, serviceError(503, "The IRC account could not be verified. Try again shortly.")
	}
	if a.RegisteredAt == nil || a.RegisteredAt.IsZero() {
		return out, serviceError(503, "The IRC account registration could not be verified.")
	}
	home := m.options.Home
	scope := strings.ToLower(net.JoinHostPort(home.Host, strconv.Itoa(home.Port)))
	sum := sha256.Sum256([]byte(scope + "\x00" + model.FoldIdentity(a.Name, "rfc1459") + "\x00" + a.RegisteredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")))
	out = model.PartyCandidate{ID: "irc:" + hex.EncodeToString(sum[:24]), ServicesAccount: a.Name, ServicesRegisteredAt: a.RegisteredAt.UTC(), Username: a.Name, DisplayName: a.Name}
	if local, _, e := m.options.Store.AccountByUsername(a.Name); e == nil && (backendAccountID == "" || backendAccountID == local.ID) {
		if profile, e := m.options.Store.LocalPersonByUsername(ctx, local.Username, Network{Host: home.Host, Port: home.Port, TLS: home.TLS}); e == nil {
			out.BackendAccountID, out.AccountID = local.ID, local.ID
			out.DisplayName, out.AvatarURL = profile.DisplayName, profile.AvatarURL
		}
	}
	if err = m.options.Store.SavePartyIdentity(out); err != nil {
		return model.PartyCandidate{}, ErrStorage
	}
	return out, nil
}

type partyActorKey struct{}

// A services principal reaches policy only after live IRC authentication. Local
// HTTP actors must still be logged in as that account on the associated server.
func (p *partyManager) actor(ctx context.Context, owner string) (model.PartyCandidate, error) {
	if actor, ok := ctx.Value(partyActorKey{}).(model.PartyCandidate); ok && actor.PrincipalID() == owner {
		return actor, nil
	}
	if strings.HasPrefix(owner, "irc:") {
		return model.PartyCandidate{}, ErrForbidden
	}
	local, e := p.store.PartyCandidate(owner)
	if e != nil {
		return local, ErrNotFound
	}
	if p.m.options.PartyAccounts == nil {
		return local, nil
	} // Isolated fake transport only.
	n, e := p.m.Network(owner, p.m.options.Home.ID)
	if e != nil || n.Status != "connected" || n.Account == "" || !strings.EqualFold(n.Account, local.Username) {
		return model.PartyCandidate{}, ErrForbidden
	}
	actor, e := p.m.ResolvePartyCandidate(ctx, n.Account, owner)
	if e != nil {
		return actor, e
	}
	if e = p.store.BindPartyPrincipal(owner, actor.ID); e != nil {
		return model.PartyCandidate{}, ErrStorage
	}
	if e = p.store.SavePartyIdentity(actor); e != nil {
		return model.PartyCandidate{}, ErrStorage
	}
	return actor, nil
}

func (p *partyManager) candidate(ctx context.Context, id string) (model.PartyCandidate, error) {
	v, e := p.store.PartyCandidate(id)
	if e != nil {
		return v, ErrNotFound
	}
	if p.m.options.PartyAccounts == nil {
		return v, nil
	}
	name := v.ServicesAccount
	if name == "" {
		name = v.Username
	}
	current, e := p.m.ResolvePartyCandidate(ctx, name, v.LocalAccountID())
	if e != nil {
		return current, e
	}
	if v.ID != "" && current.ID != v.ID {
		return model.PartyCandidate{}, ErrNotFound
	}
	return current, nil
}
func (p *partyManager) localOwner(principal string) string {
	if v, e := p.store.PartyServiceIdentity(principal); e == nil {
		local := v.LocalAccountID()
		if local == "" {
			return ""
		}
		if p.store.PartyPrincipal(local) == principal {
			return local
		}
		ctx, cancel := context.WithTimeout(p.m.ctx, 5*time.Second)
		defer cancel()
		if actor, e := p.actor(ctx, local); e == nil && actor.ID == principal {
			return local
		}
		return ""
	}
	if strings.HasPrefix(principal, "irc:") {
		return ""
	}
	return principal
}

// A services name is not sufficient to restore admission after account erasure.
// Invalid incarnations leave the durable membership before any +v is applied.
func (p *partyManager) revalidateMembers(ctx context.Context, v *storage.PartyRecord) error {
	if p.m.options.PartyAccounts == nil || v.IdentityVersion != 1 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	valid := make([]bool, len(v.Members))
	jobs := make(chan int, len(v.Members))
	for n := range v.Members {
		jobs <- n
	}
	close(jobs)
	var wg sync.WaitGroup
	var failure bool
	var mu sync.Mutex
	for range min(4, len(v.Members)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range jobs {
				member := v.Members[n]
				account, e := p.m.options.PartyAccounts.Account(ctx, member.ServicesAccount)
				if e != nil {
					if e != ergoapi.ErrNotFound {
						mu.Lock()
						failure = true
						mu.Unlock()
					}
					continue
				}
				valid[n] = account.RegisteredAt != nil && account.RegisteredAt.Equal(member.ServicesRegisteredAt) && strings.EqualFold(account.Name, member.ServicesAccount)
			}
		}()
	}
	wg.Wait()
	if failure || ctx.Err() != nil {
		return serviceError(503, "Party account registration could not be verified.")
	}
	kept := make([]model.PartyMember, 0, len(v.Members))
	for n, member := range v.Members {
		if valid[n] {
			kept = append(kept, member)
		} else {
			v.Revoked = append(v.Revoked, member.ID)
		}
	}
	if len(kept) == len(v.Members) {
		return nil
	}
	v.Members = kept
	if !v.HasMember(v.OwnerID) {
		v.OwnerID = ""
		if len(kept) > 0 {
			v.OwnerID = kept[0].ID
		}
	}
	v.State = "recovering"
	return p.save(v)
}

// Migrate only after every actor and pending invite has been verified. The
// transaction keeps retained message/reaction IDs and all unrelated bytes.
func (p *partyManager) migrateIdentities(ctx context.Context, v *storage.PartyRecord) error {
	if v.IdentityVersion == 1 || p.m.options.PartyAccounts == nil {
		return nil
	}
	mapping := map[string]model.PartyCandidate{}
	for _, member := range v.Members {
		if e := p.verifyLegacyPartyOwner(ctx, member.AccountID, member.Username); e != nil {
			return e
		}
		c, e := p.candidate(ctx, member.AccountID)
		if e != nil {
			return e
		}
		mapping[member.AccountID] = c
	}
	invites, e := p.store.PartyInvitations(v.ID)
	if e != nil {
		return ErrStorage
	}
	for _, invite := range invites {
		if _, ok := mapping[invite.Invitee]; !ok {
			c, e := p.candidate(ctx, invite.Invitee)
			if e != nil {
				return e
			}
			mapping[invite.Invitee] = c
		}
	}
	old := v.Revision
	for i, member := range v.Members {
		v.Members[i].PartyCandidate = mapping[member.AccountID]
	}
	if c, ok := mapping[v.OwnerID]; ok {
		v.OwnerID = c.ID
	}
	for i, revoked := range v.Revoked {
		if c, ok := mapping[revoked]; ok {
			v.Revoked[i] = c.ID
		}
	}
	v.IdentityVersion = 1
	v.Revision++
	if e = p.store.MigratePartyIdentities(*v, old, mapping); e != nil {
		return ErrStorage
	}
	return nil
}

func (p *partyManager) verifyLegacyPartyOwner(ctx context.Context, owner, account string) error {
	networks, e := p.store.Networks(owner)
	if e != nil {
		return ErrStorage
	}
	for _, network := range networks {
		cfg := network.Config
		if cfg.ID != p.m.options.Home.ID || !p.m.partyServer(cfg) {
			continue
		}
		username, password := cfg.SASLUser, cfg.SASLPassword
		if cfg.Auth != nil {
			username = cfg.Auth.Account
			if cfg.Auth.Password != nil {
				password = *cfg.Auth.Password
			}
		}
		if !strings.EqualFold(username, account) || password == "" {
			continue
		}
		matches, e := p.m.options.PartyAccounts.CheckPassword(ctx, account, password)
		if e != nil {
			return serviceError(503, "The party account identity could not be verified.")
		}
		if matches {
			return nil
		}
	}
	return serviceError(503, "The party account identity needs a verified associated-server login.")
}

func (m *Manager) CreatePartyWithServices(ctx context.Context, owner, client, name, description string, accounts []string) (model.Party, error) {
	if len(accounts) > 31 {
		return model.Party{}, fmt.Errorf("a party can contain at most 32 members")
	}
	ids := make([]string, 0, len(accounts))
	for _, account := range accounts {
		v, e := m.ResolvePartyCandidate(ctx, account, "")
		if e != nil {
			return model.Party{}, e
		}
		ids = append(ids, v.ID)
	}
	return m.CreateParty(ctx, owner, client, name, description, ids)
}
func (m *Manager) InvitePartyServices(ctx context.Context, owner, party, account string) (model.Invitation, error) {
	v, e := m.ResolvePartyCandidate(ctx, account, "")
	if e != nil {
		return model.Invitation{}, e
	}
	return m.InviteParty(ctx, owner, party, v.ID)
}
