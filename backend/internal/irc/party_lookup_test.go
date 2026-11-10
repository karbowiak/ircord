package irc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/model"
)

func partyLookupFixture(t *testing.T, m *Manager, lookup func(string) (bool, int)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ns/info" || r.Method != http.MethodPost {
			t.Errorf("unexpected account directory operation: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var input struct{ AccountName string }
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		exists, status := lookup(input.AccountName)
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": exists, "accountName": input.AccountName, "registeredAt": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	}))
	t.Cleanup(server.Close)
	client, err := ergoapi.New(ergoapi.Options{Server: "irc.fixture:6697", URL: server.URL, Token: strings.Repeat("fixture-", 8)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	m.options.PartyAccounts = client
	return server
}

func partyDirectoryLookupFixture(t *testing.T, m *Manager, status func() int) (*httptest.Server, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	w := m.space("alice")
	w.mu.Lock()
	w.networks["home"] = Network{ID: "home", Host: "irc.fixture", Port: 6697, TLS: true, Nick: "Alice", Account: "alice", Status: "connected"}
	w.mu.Unlock()
	var directories, checks atomic.Int32
	registered := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status() != http.StatusOK {
			w.WriteHeader(status())
			return
		}
		switch r.URL.Path {
		case "/v1/ns/list":
			directories.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "accounts": []any{
				map[string]any{"success": true, "accountName": "alice"},
				map[string]any{"success": true, "accountName": "bob"},
				map[string]any{"success": true, "accountName": "PeterPowers"},
				map[string]any{"success": true, "accountName": "ppetermann"},
			}})
		case "/v1/ns/info":
			checks.Add(1)
			var input struct{ AccountName string }
			_ = json.NewDecoder(r.Body).Decode(&input)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "accountName": input.AccountName, "registeredAt": registered})
		default:
			t.Errorf("unexpected lookup source %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client, err := ergoapi.New(ergoapi.Options{Server: "irc.fixture:6697", URL: server.URL, Token: strings.Repeat("fixture-", 8)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	m.options.PartyAccounts = client
	return server, &directories, &checks
}

func TestPartyCandidatesShareRegisteredDirectoryAndKeepLocalIDsDistinct(t *testing.T) {
	m, s, _ := partyFixture(t)
	if _, err := s.Commit("bob", "member.updated", model.Member{ID: "bob", DisplayName: "Peter Jensen"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAccount(partyControlOwner, "controller"); err != nil {
		t.Fatal(err)
	}
	_, directories, checks := partyDirectoryLookupFixture(t, m, func() int { return http.StatusOK })
	for _, search := range []string{"bo", "peter", ""} {
		items, err := m.PartyCandidates("alice", search)
		want := 3
		if search == "bo" {
			want = 1
		}
		if err != nil || len(items) != want {
			t.Fatalf("search %q = %+v, %v", search, items, err)
		}
		for _, item := range items {
			if item.ServicesAccount == "bob" {
				if item.BackendAccountID != "bob" || item.AccountID != "bob" || item.DisplayName != "Peter Jensen" {
					t.Fatal("verified local profile was lost", item)
				}
			} else if item.AccountID != "" || item.BackendAccountID != "" {
				t.Fatal("native registered account received a fake local ID", item)
			}
			if !strings.HasPrefix(item.ID, "irc:") || item.ServicesRegisteredAt.IsZero() || item.ID == item.ServicesAccount {
				t.Fatal("candidate bypassed the canonical principal resolver", item)
			}
		}
	}
	if checks.Load() != 3 || directories.Load() != 1 {
		t.Fatalf("typing refetched registered source data: directory=%d, checks=%d", directories.Load(), checks.Load())
	}
	items, err := m.PartyCandidates("alice", "no local matches")
	if err != nil || len(items) != 0 || checks.Load() != 3 || directories.Load() != 1 {
		t.Fatal("empty matches refetched source data", err)
	}
	if _, err = m.PartyCandidates("alice", strings.Repeat("ø", 100)); err != nil {
		t.Fatal("100 UTF-8 characters were rejected", err)
	}
	for _, search := range []string{strings.Repeat("ø", 101), string([]byte{0xff})} {
		if _, err = m.PartyCandidates("alice", search); err == nil {
			t.Fatal("invalid search was accepted")
		}
	}
}

func TestPartyCandidateLookupFailuresRemainUnavailableAndRetry(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			m, _, _ := partyFixture(t)
			var upstreamStatus atomic.Int32
			upstreamStatus.Store(int32(status))
			partyDirectoryLookupFixture(t, m, func() int { return int(upstreamStatus.Load()) })
			items, err := m.PartyCandidates("alice", "bo")
			serviceStatus(t, err, http.StatusServiceUnavailable)
			if items != nil {
				t.Fatal("failed lookup returned unvalidated candidates")
			}
			upstreamStatus.Store(http.StatusOK)
			items, err = m.PartyCandidates("alice", "bo")
			if err != nil || len(items) != 1 {
				t.Fatal("lookup failure remained cached after recovery", err)
			}
		})
	}
	t.Run("connection refused", func(t *testing.T) {
		m, _, _ := partyFixture(t)
		server, _, _ := partyDirectoryLookupFixture(t, m, func() int { return http.StatusOK })
		server.Close()
		items, err := m.PartyCandidates("alice", "bo")
		serviceStatus(t, err, http.StatusServiceUnavailable)
		if items != nil {
			t.Fatal("unreachable associated API returned unvalidated candidates")
		}
	})
}

func TestPartyAccountValidationStillAppliesToCreateInviteAndAccept(t *testing.T) {
	m, s, transport := partyFixture(t)
	for _, owner := range []string{"alice", "bob"} {
		w := m.space(owner)
		w.mu.Lock()
		w.networks["home"] = Network{ID: "home", Host: "irc.fixture", Port: 6697, TLS: true, Nick: owner, Account: owner, Status: "connected"}
		w.mu.Unlock()
	}
	var bobAvailable, creatorAvailable atomic.Bool
	creatorAvailable.Store(true)
	partyLookupFixture(t, m, func(username string) (bool, int) {
		if username == "alice" {
			return creatorAvailable.Load(), http.StatusOK
		}
		return username == "bob" && bobAvailable.Load(), http.StatusOK
	})
	if _, err := m.CreateParty(context.Background(), "alice", "invalid-invitee", "", "", []string{"bob"}); err == nil {
		t.Fatal("create accepted an account absent from the associated server")
	}
	creatorAvailable.Store(false)
	_, err := m.CreateParty(context.Background(), "alice", "invalid-creator", "", "", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("missing creator was not rejected", err)
	}
	if transport.calls != 0 {
		t.Fatal("invalid accounts reached party provisioning")
	}
	creatorAvailable.Store(true)
	bobAvailable.Store(true)
	party := createFixtureParty(t, m, "bob")
	invitation := fixturePartyInvite(t, s, party.ID, "bob")
	bobAvailable.Store(false)
	if _, err = m.AcceptInvitation(context.Background(), "bob", invitation.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("accept did not revalidate the associated account", err)
	}
	if _, err = m.InviteParty(context.Background(), "alice", party.ID, "bob"); !errors.Is(err, ErrNotFound) {
		t.Fatal("invite did not revalidate the associated account", err)
	}
	record, err := s.Party(party.ID)
	if err != nil || record.HasMember(s.PartyPrincipal("bob")) {
		t.Fatal("rejected acceptance changed membership", err)
	}
	bobAvailable.Store(true)
	if _, err = m.AcceptInvitation(context.Background(), "bob", invitation.ID); err != nil {
		t.Fatal("valid account could not accept after recovery", err)
	}
}

func TestPartyTargetedAccountChecksAreBoundedAndCancelable(t *testing.T) {
	m, _, _ := partyFixture(t)
	var active, peak atomic.Int32
	partyLookupFixture(t, m, func(string) (bool, int) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		time.Sleep(10 * time.Millisecond)
		return true, http.StatusOK
	})
	candidates := make([]model.PartyCandidate, 50)
	for i := range candidates {
		candidates[i].Username = "fixture"
	}
	if _, err := m.parties.associatedAccounts(context.Background(), candidates); err != nil {
		t.Fatal(err)
	}
	if peak.Load() > 4 {
		t.Fatalf("account lookup exceeded four concurrent requests: %d", peak.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := m.parties.associatedAccounts(ctx, candidates)
	serviceStatus(t, err, http.StatusServiceUnavailable)
}
