package irc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lrstanley/girc"
	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

const peopleSourceLimit = 4096

type peopleCacheEntry struct {
	ready     chan struct{}
	at        time.Time
	expires   time.Time
	names     []string
	person    model.Person
	candidate model.PartyCandidate
	found     bool
	more      bool
	err       error
}

// Cache entries are isolated by owner, network endpoint, purpose and source/query.
// Pending work is coalesced; failures and cancelled work are never retained.
type peopleCache struct {
	mu      sync.Mutex
	entries map[string]*peopleCacheEntry
	limit   int
}

func (p *peopleCache) lookup(ctx context.Context, key string, ttl time.Duration, load func(*peopleCacheEntry) error) (*peopleCacheEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	if p.entries == nil {
		p.entries = map[string]*peopleCacheEntry{}
	}
	if v := p.entries[key]; v != nil {
		select {
		case <-v.ready:
			if time.Now().Before(v.expires) {
				p.mu.Unlock()
				return v, nil
			}
			delete(p.entries, key)
		default:
			p.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-v.ready:
				return v, v.err
			}
		}
	}
	limit := p.limit
	if limit == 0 {
		limit = 32
	}
	if len(p.entries) >= limit {
		oldestKey := ""
		var oldest time.Time
		for k, v := range p.entries {
			select {
			case <-v.ready:
				if oldestKey == "" || v.at.Before(oldest) {
					oldestKey, oldest = k, v.at
				}
			default:
			}
		}
		if oldestKey == "" {
			p.mu.Unlock()
			return nil, ErrBusy
		}
		delete(p.entries, oldestKey)
	}
	v := &peopleCacheEntry{ready: make(chan struct{}), at: time.Now()}
	p.entries[key] = v
	p.mu.Unlock()
	v.err = load(v)
	if ctx.Err() != nil {
		v.err = ctx.Err()
	}
	p.mu.Lock()
	if v.err != nil {
		delete(p.entries, key)
	} else {
		v.expires = time.Now().Add(ttl)
	}
	close(v.ready)
	p.mu.Unlock()
	return v, v.err
}

func (m *Manager) associatedPeopleNetwork(n Network) bool {
	c := m.options.PartyAccounts
	return c != nil && strings.EqualFold(net.JoinHostPort(n.Host, strconv.Itoa(n.Port)), c.Server())
}

func peopleKey(network, kind, value string) string {
	return network + ":" + kind + ":" + value
}

func accountPurpose(purpose string) bool { return purpose == "services" || purpose == "party" }

// LookupPeople reads only the caller's selected network and authorized context.
// It never authorizes a subsequent mutation, which must recheck its own policy.
func (m *Manager) LookupPeople(ctx context.Context, owner string, q model.PeopleQuery) (model.PeoplePage, error) {
	page := model.PeoplePage{Items: []model.Person{}, Source: "known", Status: "ready"}
	if err := ctx.Err(); err != nil {
		return page, err
	}
	if owner == "" || !utf8.ValidString(q.Query) || utf8.RuneCountInString(q.Query) > 100 || !clean(q.Query, 400) {
		return page, fmt.Errorf("invalid people lookup")
	}
	switch q.Purpose {
	case "mention", "command", "mask", "history", "dm", "services", "party":
	default:
		return page, fmt.Errorf("choose a people lookup purpose")
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.Limit < 1 || q.Limit > 50 {
		return page, fmt.Errorf("people limit must be between 1 and 50")
	}
	q.Query = strings.TrimSpace(q.Query)
	conversationOnly := q.Purpose == "mention" || q.Purpose == "command" || q.Purpose == "mask"
	if conversationOnly && q.ConversationID == "" {
		return page, fmt.Errorf("choose a conversation for this lookup")
	}
	w := m.space(owner)
	w.mu.Lock()
	n, ok := w.networks[q.NetworkID]
	if !ok || w.deleted {
		w.mu.Unlock()
		return page, ErrNotFound
	}
	if w.storageErr != nil {
		w.mu.Unlock()
		return page, ErrStorage
	}
	var conv Conversation
	if q.ConversationID != "" {
		conv, ok = w.conversations[q.ConversationID]
		if !ok || conv.NetworkID != q.NetworkID {
			w.mu.Unlock()
			return page, ErrNotFound
		}
		if err := m.partyAccess(owner, conv.ID); err != nil {
			w.mu.Unlock()
			return page, err
		}
	}
	c := w.connections[q.NetworkID]
	mapping := n.ISupport["CASEMAPPING"]
	fold := func(v string) string { return model.FoldIdentity(v, mapping) }
	ids := map[string]bool{}
	if conversationOnly {
		for _, id := range conv.MemberIDs {
			ids[id] = true
		}
		messages := w.messages[conv.ID]
		for _, msg := range messages[max(0, len(messages)-500):] {
			ids[msg.SenderID] = true
		}
	} else {
		for id := range w.members {
			if q.ConversationID == "" && strings.HasPrefix(id, q.NetworkID+":") {
				ids[id] = true
			}
		}
		for _, v := range w.conversations {
			if v.NetworkID == q.NetworkID && (q.ConversationID == "" || q.ConversationID == v.ID) {
				if v.Party != nil && m.partyAccess(owner, v.ID) != nil {
					continue
				}
				for _, id := range v.MemberIDs {
					ids[id] = true
				}
				if q.ConversationID != "" {
					for _, msg := range w.messages[v.ID] {
						ids[msg.SenderID] = true
					}
				}
			}
		}
	}
	// Own identity keeps its stable ID and uses this network's profile.
	if n.Nick != "" && !conversationOnly {
		ids[owner] = true
	}
	orderedIDs := make([]string, 0, len(ids))
	for id := range ids {
		orderedIDs = append(orderedIDs, id)
	}
	sort.Strings(orderedIDs)
	people := make(map[string]model.Person, min(len(orderedIDs), peopleSourceLimit))
	for i, id := range orderedIDs {
		if i >= peopleSourceLimit {
			page.More = true
			break
		}
		v, exists := w.members[id]
		if !exists || v.Ignored {
			continue
		}
		if id == owner {
			v = w.networkSelf(q.NetworkID)
			v.Nick, v.Account = n.Nick, n.Account
		}
		person := personFromMember(v, q.NetworkID)
		if n.Status != "connected" && person.Presence != "offline" {
			person.Presence = "unknown"
		}
		people[person.Key] = person
	}
	ownProfile := w.networkSelf(q.NetworkID)
	w.mu.Unlock()
	if q.ConversationID != "" && m.options.Store != nil {
		authors, err := m.options.Store.RecentPeople(ctx, owner, conv.ID)
		if err != nil {
			if ctx.Err() != nil {
				return page, ctx.Err()
			}
			return page, ErrStorage
		}
		for _, v := range authors {
			if v.ID == owner {
				v = ownProfile
			}
			key := peopleKey(q.NetworkID, "member", v.ID)
			if _, exists := people[key]; !exists && !v.Ignored {
				if len(people) >= peopleSourceLimit {
					page.More = true
					break
				}
				v.Status = "offline"
				people[key] = personFromMember(v, q.NetworkID)
			}
		}
	}
	associated := m.associatedPeopleNetwork(n)
	if q.Purpose == "party" && (!associated || n.ID != m.options.Home.ID || m.PartyCapability(owner) == nil) {
		return page, ErrUnsupported
	}
	cacheScope := owner + "\x00" + n.ID + "\x00" + net.JoinHostPort(n.Host, strconv.Itoa(n.Port)) + "\x00" + strconv.FormatBool(n.TLS) + "\x00" + q.Purpose + "\x00"
	if q.Purpose == "dm" && validateNickname(q.Query, defaultNickLength) == nil && !hasCurrentNickname(people, fold(q.Query), fold) {
		resolved, err := m.people.lookup(ctx, cacheScope+"whois:"+fold(q.Query), 10*time.Second, func(entry *peopleCacheEntry) error {
			person, found, err := m.exactPerson(ctx, n, c, q.Query, associated)
			entry.person, entry.found = person, found
			return err
		})
		if ctx.Err() != nil {
			return page, ctx.Err()
		}
		if err != nil {
			page.Status = "partial"
		} else if resolved.found {
			person := resolved.person
			for key, prior := range people {
				if fold(prior.Nickname) == fold(person.Nickname) {
					person.Key, person.MemberID = prior.Key, prior.MemberID
					delete(people, key)
				}
			}
			people[person.Key] = person
		}
	}
	if accountPurpose(q.Purpose) {
		people = mergeAccountPeople(people, q.NetworkID, q.Query, mapping, fold)
		if associated {
			page.Source = "registered"
			directory, err := m.people.lookup(ctx, cacheScope+"directory", 30*time.Second, func(entry *peopleCacheEntry) error {
				lookupCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
				defer cancel()
				var err error
				entry.names, entry.more, err = m.options.PartyAccounts.RegisteredNames(lookupCtx, peopleSourceLimit)
				return err
			})
			if ctx.Err() != nil {
				return page, ctx.Err()
			}
			if err != nil {
				page.Status = "unavailable"
				for key, person := range people {
					if q.Purpose == "party" {
						person.Eligible, person.Reason = false, "identity_unverified"
					}
					people[key] = person
				}
			} else {
				page.More = page.More || directory.more
				registered := map[string]bool{}
				local := map[string]storage.LocalPerson{}
				if m.options.Store != nil {
					matches, more, err := m.options.Store.LocalPeople(ctx, q.Query, 51, n)
					if err != nil {
						if ctx.Err() != nil {
							return page, ctx.Err()
						}
						return page, ErrStorage
					}
					page.More = page.More || more
					for _, v := range matches {
						local[fold(v.Username)] = v
					}
				}
				for _, name := range directory.names {
					if strings.HasPrefix(strings.ToLower(name), "ircordsvc") {
						continue
					}
					registered[fold(name)] = true
					key := peopleKey(q.NetworkID, "account", fold(name))
					person, exists := people[key]
					if !exists {
						person = model.Person{Key: key, NetworkID: q.NetworkID, ServicesAccount: name, DisplayName: name, Presence: "unknown"}
					}
					person.Eligible, person.Reason = true, ""
					if profile, ok := local[fold(name)]; ok {
						person.BackendAccountID, person.DisplayName, person.AvatarURL = profile.BackendAccountID, profile.DisplayName, profile.AvatarURL
					}
					people[key] = person
				}
				for key, person := range people {
					if !registered[fold(person.ServicesAccount)] {
						person.Eligible, person.Reason = false, "account_unknown"
						if directory.more {
							person.Reason = "identity_unverified"
						}
						people[key] = person
					}
				}
			}
		}
	}
	for _, person := range people {
		if q.Purpose == "history" {
			person.Eligible, person.Reason = person.MemberID != "", ""
		}
		if q.Purpose == "party" && person.BackendAccountID == owner {
			continue
		}
		if personRank(person, q.Query, mapping) < 99 {
			page.Items = append(page.Items, person)
		}
	}
	sort.Slice(page.Items, func(i, j int) bool {
		a, b := page.Items[i], page.Items[j]
		x, y := personRank(a, q.Query, mapping), personRank(b, q.Query, mapping)
		if x != y {
			return x < y
		}
		if a.Eligible != b.Eligible {
			return a.Eligible
		}
		if fold(a.DisplayName) != fold(b.DisplayName) {
			return fold(a.DisplayName) < fold(b.DisplayName)
		}
		return a.Key < b.Key
	})
	if len(page.Items) > q.Limit {
		page.Items, page.More = page.Items[:q.Limit], true
	}
	// Enrich only verified associated accounts, and never infer a nickname.
	if associated && m.options.Store != nil {
		for i := range page.Items {
			person := &page.Items[i]
			if person.ServicesAccount == "" || !person.Eligible {
				continue
			}
			profile, err := m.options.Store.LocalPersonByUsername(ctx, person.ServicesAccount, n)
			if err == nil {
				person.BackendAccountID = profile.BackendAccountID
			} else if !errors.Is(err, storage.ErrNotFound) {
				if ctx.Err() != nil {
					return page, ctx.Err()
				}
				return page, ErrStorage
			}
		}
	}
	if q.Purpose == "party" {
		items := page.Items[:0]
		for _, person := range page.Items {
			if person.BackendAccountID != owner {
				items = append(items, person)
			}
		}
		page.Items = items
	}
	if page.Status == "unavailable" && len(page.Items) > 0 {
		page.Status = "partial"
	}
	return page, ctx.Err()
}

func personFromMember(v Member, network string) model.Person {
	p := model.Person{Key: peopleKey(network, "member", v.ID), NetworkID: network, MemberID: v.ID, Nickname: v.Nick, ServicesAccount: v.Account, DisplayName: v.DisplayName, AvatarURL: v.AvatarURL, Presence: v.Status, Eligible: v.Nick != ""}
	if p.ServicesAccount == "*" {
		p.ServicesAccount = ""
	}
	if p.DisplayName == "" {
		p.DisplayName = p.Nickname
	}
	if p.Presence != "online" && p.Presence != "away" && p.Presence != "offline" {
		p.Presence = "unknown"
	}
	if !p.Eligible {
		p.Reason = "nickname_unknown"
	}
	return p
}

func personRank(p model.Person, query, mapping string) int {
	identities := append([]string{p.Nickname, p.ServicesAccount}, p.Nicknames...)
	return model.PersonMatchRank(query, mapping, p.DisplayName, identities...)
}

func hasCurrentNickname(people map[string]model.Person, nick string, fold func(string) string) bool {
	for _, person := range people {
		if fold(person.Nickname) == nick && (person.Presence == "online" || person.Presence == "away") {
			return true
		}
	}
	return false
}

func mergeAccountPeople(people map[string]model.Person, network, query, mapping string, fold func(string) string) map[string]model.Person {
	out := map[string]model.Person{}
	for _, person := range people {
		if person.ServicesAccount == "" {
			person.Eligible, person.Reason = false, "account_unknown"
			out[person.Key] = person
			continue
		}
		person.Key = peopleKey(network, "account", fold(person.ServicesAccount))
		person.Eligible, person.Reason = true, ""
		if prior, ok := out[person.Key]; ok {
			aliases := append(prior.Nicknames, person.Nickname)
			rank, priorRank := model.PersonMatchRank(query, mapping, person.DisplayName, person.Nickname), model.PersonMatchRank(query, mapping, prior.DisplayName, prior.Nickname)
			current := person.Presence == "online" || person.Presence == "away"
			priorCurrent := prior.Presence == "online" || prior.Presence == "away"
			if current && !priorCurrent || current == priorCurrent && (rank < priorRank || rank == priorRank && person.Nickname < prior.Nickname) {
				person.Nicknames = aliases
			} else {
				person = prior
				person.Nicknames = aliases
			}
		} else if person.Nickname != "" {
			person.Nicknames = []string{person.Nickname}
		}
		sort.Strings(person.Nicknames)
		person.Nicknames = slices.Compact(person.Nicknames)
		out[person.Key] = person
	}
	return out
}

func (m *Manager) exactPerson(ctx context.Context, n Network, c *connection, nick string, associated bool) (model.Person, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if associated {
		who, err := m.options.PartyAccounts.User(ctx, nick)
		if errors.Is(err, ergoapi.ErrNotFound) {
			return model.Person{}, false, nil
		}
		if err != nil {
			return model.Person{}, false, err
		}
		if model.FoldIdentity(who.Nickname, n.ISupport["CASEMAPPING"]) != model.FoldIdentity(nick, n.ISupport["CASEMAPPING"]) || validateNickname(who.Nickname, 0) != nil {
			return model.Person{}, false, fmt.Errorf("associated IRC server returned a different nickname")
		}
		p := model.Person{Key: peopleKey(n.ID, "nick", model.FoldIdentity(who.Nickname, n.ISupport["CASEMAPPING"])), NetworkID: n.ID, Nickname: who.Nickname, ServicesAccount: who.Account, DisplayName: who.Realname, Presence: "online", Eligible: true}
		if p.DisplayName == "" {
			p.DisplayName = p.Nickname
		}
		if who.Away != "" {
			p.Presence = "away"
		}
		if p.ServicesAccount == "*" {
			p.ServicesAccount = ""
		}
		return p, true, nil
	}
	return ordinaryWhois(ctx, n, c, nick)
}

func ordinaryWhois(ctx context.Context, n Network, c *connection, nick string) (model.Person, bool, error) {
	if c == nil {
		return model.Person{}, false, ErrDisconnected
	}
	c.mu.Lock()
	client, ready := c.client, c.ready
	c.mu.Unlock()
	if !ready || client == nil {
		return model.Person{}, false, ErrDisconnected
	}
	// Global connection slots bound even unrelated queries and HTTP callers.
	select {
	case c.manager.checkSlots <- struct{}{}:
		defer func() { <-c.manager.checkSlots }()
	default:
		return model.Person{}, false, ErrBusy
	}
	var mu sync.Mutex
	p := model.Person{NetworkID: n.ID, Presence: "online", Eligible: true}
	done := make(chan struct{}, 1)
	fold := func(v string) string { return model.FoldIdentity(v, n.ISupport["CASEMAPPING"]) }
	handler := client.Handlers.Add(girc.ALL_EVENTS, func(_ *girc.Client, e girc.Event) {
		if len(e.Params) < 2 || fold(e.Params[1]) != fold(nick) {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch e.Command {
		case "311":
			p.Nickname, p.DisplayName = e.Params[1], e.Last()
		case "330":
			if len(e.Params) >= 3 {
				p.ServicesAccount = e.Params[2]
			}
		case "301":
			p.Presence = "away"
		case "318", "401":
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
	defer client.Handlers.Remove(handler)
	if err := c.enqueue(&girc.Event{Command: "WHOIS", Params: []string{nick}}); err != nil {
		return model.Person{}, false, err
	}
	select {
	case <-ctx.Done():
		return model.Person{}, false, ctx.Err()
	case <-c.ctx.Done():
		return model.Person{}, false, ErrDisconnected
	case <-done:
		mu.Lock()
		defer mu.Unlock()
		p.Key = peopleKey(n.ID, "nick", fold(p.Nickname))
		if p.DisplayName == "" {
			p.DisplayName = p.Nickname
		}
		return p, p.Nickname != "", nil
	}
}
