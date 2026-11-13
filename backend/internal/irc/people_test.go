package irc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ergoapi "ircord/backend/internal/ergo"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func peopleFixture(t *testing.T) (*Manager, *workspace) {
	t.Helper()
	m := New(Options{})
	t.Cleanup(m.Close)
	w := m.space("caller")
	w.networks["one"] = Network{ID: "one", Host: "irc.fixture", Port: 6697, TLS: true, Status: "connected", Nick: "Caller", ISupport: map[string]string{"CASEMAPPING": "ascii"}}
	w.networks["two"] = Network{ID: "two", Host: "other.fixture", Port: 6697, TLS: true, Status: "connected", Nick: "OtherCaller"}
	w.members["one:a"] = Member{ID: "one:a", Nick: "PeterPowers", Account: "shared", Username: "not-a-login", DisplayName: "Peter Jensen", Status: "online"}
	w.members["one:b"] = Member{ID: "one:b", Nick: "OtherNick", Account: "shared", DisplayName: "Other person", Status: "online"}
	w.members["one:old"] = Member{ID: "one:old", Nick: "OfflineNick", Account: "oldaccount", DisplayName: "Offline person", Status: "offline"}
	w.members["two:secret"] = Member{ID: "two:secret", Nick: "SecretNick", Account: "secret", DisplayName: "Secret name", Status: "online"}
	w.conversations["channel"] = Conversation{ID: "channel", NetworkID: "one", Joined: true, MemberIDs: []string{"one:a", "one:b"}}
	w.conversations["secret"] = Conversation{ID: "secret", NetworkID: "two", Joined: true, MemberIDs: []string{"two:secret"}}
	w.messages["channel"] = []Message{{ID: "old", SenderID: "one:old"}}
	return m, w
}

func peopleServer(t *testing.T, m *Manager, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := ergoapi.New(ergoapi.Options{Server: "irc.fixture:6697", URL: server.URL, Token: strings.Repeat("fixture-", 8)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	m.options.AssociatedServer = client
}

func TestPeopleNetworkConversationAndNicknameIdentity(t *testing.T) {
	m, _ := peopleFixture(t)
	page, err := m.LookupPeople(context.Background(), "caller", model.PeopleQuery{NetworkID: "one", Purpose: "mention", ConversationID: "channel"})
	if err != nil || len(page.Items) != 3 {
		t.Fatalf("conversation lookup: %+v, %v", page, err)
	}
	keys := map[string]bool{}
	for _, p := range page.Items {
		if p.Nickname == "SecretNick" || p.ServicesAccount == "not-a-login" || p.BackendAccountID != "" || keys[p.Key] {
			t.Fatalf("scope, identity or alias collision: %+v", p)
		}
		keys[p.Key] = true
	}
	for _, q := range []model.PeopleQuery{
		{NetworkID: "one", Purpose: "mention", ConversationID: "secret"},
		{NetworkID: "missing", Purpose: "history"},
	} {
		if _, err = m.LookupPeople(context.Background(), "caller", q); !errors.Is(err, ErrNotFound) {
			t.Fatal("lookup crossed authorization scope", q, err)
		}
	}
	page, err = m.LookupPeople(context.Background(), "different-caller", model.PeopleQuery{NetworkID: "one", Purpose: "history"})
	if !errors.Is(err, ErrNotFound) || len(page.Items) != 0 {
		t.Fatal("lookup crossed account scope", page, err)
	}
	page, err = m.LookupPeople(context.Background(), "caller", model.PeopleQuery{NetworkID: "one", Purpose: "history", Query: "Offline"})
	if err != nil || len(page.Items) != 1 || page.Items[0].Presence != "offline" || page.Items[0].MemberID != "one:old" {
		t.Fatal("offline history identity was lost", page, err)
	}
}

func TestPeopleRegisteredNamesDiscoverNativeAccountsAndMergeOnlyVerifiedAliases(t *testing.T) {
	m, _ := peopleFixture(t)
	var calls atomic.Int32
	peopleServer(t, m, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ns/list" {
			t.Errorf("unexpected source %s", r.URL.Path)
		}
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "accounts": []any{
			map[string]any{"success": true, "accountName": "shared", "channels": []string{"#secret"}},
			map[string]any{"success": true, "accountName": "PeterNative", "registeredAt": time.Now()},
			map[string]any{"success": true, "accountName": "ppetermann"},
		}})
	})
	for _, query := range []string{"Peter", "ppeter", "shared"} {
		page, err := m.LookupPeople(context.Background(), "caller", model.PeopleQuery{NetworkID: "one", Purpose: "services", Query: query})
		if err != nil || page.Source != "registered" || page.Status != "ready" || len(page.Items) == 0 {
			t.Fatalf("registered search %q: %+v, %v", query, page, err)
		}
		for _, p := range page.Items {
			if p.BackendAccountID != "" || !p.Eligible {
				t.Fatal("native account required or invented a local row", p)
			}
			if p.ServicesAccount == "PeterNative" && (p.Nickname != "" || p.Presence != "unknown") {
				t.Fatal("account name became a nickname or online claim", p)
			}
			if p.ServicesAccount == "shared" && len(p.Nicknames) != 2 {
				t.Fatal("verified aliases were not merged", p)
			}
		}
	}
	if calls.Load() != 1 {
		t.Fatal("directory fetched again while typing", calls.Load())
	}
	page, err := m.LookupPeople(context.Background(), "caller", model.PeopleQuery{NetworkID: "two", Purpose: "services", Query: "Peter"})
	if err != nil || len(page.Items) != 0 || calls.Load() != 1 || page.Source != "known" {
		t.Fatal("associated directory leaked into another network", page, err)
	}
	page, err = m.LookupPeople(context.Background(), "caller", model.PeopleQuery{NetworkID: "one", Purpose: "services", Query: "missing"})
	if err != nil || len(page.Items) != 0 || page.Status != "ready" {
		t.Fatal("unknown account was not distinct from an outage", page, err)
	}
}

func TestPeopleDirectoryFailuresRetryAndCancellationIsNotCached(t *testing.T) {
	m, _ := peopleFixture(t)
	var fail atomic.Bool
	fail.Store(true)
	var calls atomic.Int32
	peopleServer(t, m, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "accounts": []any{map[string]any{"success": true, "accountName": "native"}}})
	})
	q := model.PeopleQuery{NetworkID: "one", Purpose: "services", Query: "native"}
	page, err := m.LookupPeople(context.Background(), "caller", q)
	if err != nil || page.Status != "unavailable" || len(page.Items) != 0 {
		t.Fatal("directory failure became unknown account", page, err)
	}
	fail.Store(false)
	page, err = m.LookupPeople(context.Background(), "caller", q)
	if err != nil || page.Status != "ready" || len(page.Items) != 1 || calls.Load() != 2 {
		t.Fatal("source failure was cached", page, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = m.LookupPeople(ctx, "caller", q); !errors.Is(err, context.Canceled) || calls.Load() != 2 {
		t.Fatal("cancelled lookup continued", err)
	}
}

func TestPeopleRegisteredProfileVerificationAndNativePartyEligibility(t *testing.T) {
	m, _ := peopleFixture(t)
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for _, account := range []struct{ id, username string }{{"caller", "caller"}, {"local-bob", "bob"}, {"NativeAccount", "unrelated-login"}} {
		if _, err = s.CreateAccount(account.id, account.username); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Commit("local-bob", "network.updated", Network{ID: "home", Host: "irc.fixture", Port: 6697, TLS: true, Auth: model.NetworkAuth{Method: "sasl-plain", Account: "bob"}, Profile: &Member{ID: "local-bob", DisplayName: "Peter Jensen"}}); err != nil {
		t.Fatal(err)
	}
	m.options.Store, m.options.Home = s, NetworkConfig{ID: "one", Host: "irc.fixture", Port: 6697, TLS: true}
	m.parties = &partyManager{m: m, store: s}
	peopleServer(t, m, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "accounts": []any{
			map[string]any{"success": true, "accountName": "bob"},
			map[string]any{"success": true, "accountName": "NativeAccount"},
		}})
	})
	page, err := m.LookupPeople(context.Background(), "caller", model.PeopleQuery{NetworkID: "one", Purpose: "party", Query: "Peter"})
	var bob model.Person
	for _, person := range page.Items {
		if person.ServicesAccount == "bob" {
			bob = person
		}
	}
	if err != nil || !bob.Eligible || bob.BackendAccountID != "local-bob" || bob.DisplayName != "Peter Jensen" || bob.Nickname != "" {
		t.Fatal("verified local profile did not participate in registered matching", page, err)
	}
	page, err = m.LookupPeople(context.Background(), "caller", model.PeopleQuery{NetworkID: "one", Purpose: "party", Query: "Native"})
	if err != nil || len(page.Items) != 1 || !page.Items[0].Eligible || page.Items[0].BackendAccountID != "" || page.Items[0].ServicesAccount != "NativeAccount" {
		t.Fatal("party discovery required or invented a local account", page, err)
	}
}

func TestPeopleExactDMResolvesNicknameAccountMismatchOnlyOnSelectedNetwork(t *testing.T) {
	m, _ := peopleFixture(t)
	var calls atomic.Int32
	peopleServer(t, m, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/whois" {
			t.Errorf("unexpected source %s", r.URL.Path)
		}
		calls.Add(1)
		var query struct{ Nickname string }
		_ = json.NewDecoder(r.Body).Decode(&query)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "present": query.Nickname == "DifferentNick", "nickname": "DifferentNick", "account": "ServicesLogin", "realname": "Display name"})
	})
	q := model.PeopleQuery{NetworkID: "one", Purpose: "dm", Query: "DifferentNick"}
	page, err := m.LookupPeople(context.Background(), "caller", q)
	if err != nil || len(page.Items) != 1 || page.Items[0].Nickname != "DifferentNick" || page.Items[0].ServicesAccount != "ServicesLogin" || page.Items[0].BackendAccountID != "" {
		t.Fatal("exact nickname/account mapping failed", page, err)
	}
	page, err = m.LookupPeople(context.Background(), "caller", q)
	if err != nil || len(page.Items) != 1 || calls.Load() != 1 {
		t.Fatal("exact lookup was not cached", page, err)
	}
	q.Query = "ServicesLogin"
	page, err = m.LookupPeople(context.Background(), "caller", q)
	if err != nil || len(page.Items) != 0 || page.Status != "ready" {
		t.Fatal("services account was synthesized as a nickname", page, err)
	}
	q.NetworkID, q.Query = "two", "DifferentNick"
	page, err = m.LookupPeople(context.Background(), "caller", q)
	if err != nil || len(page.Items) != 0 || calls.Load() != 2 || page.Status != "partial" {
		t.Fatal("associated WHOIS crossed the selected network", page, err)
	}
}

func TestPeopleCancelledTypingStopsUpstreamAndFreshKnownPeopleStayFresh(t *testing.T) {
	m, w := peopleFixture(t)
	started, upstreamStopped := make(chan struct{}), make(chan struct{})
	peopleServer(t, m, func(out http.ResponseWriter, r *http.Request) {
		// Reading the request body lets net/http observe a later client disconnect.
		var body any
		_ = json.NewDecoder(r.Body).Decode(&body)
		close(started)
		<-r.Context().Done()
		close(upstreamStopped)
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := m.LookupPeople(ctx, "caller", model.PeopleQuery{NetworkID: "one", Purpose: "dm", Query: "CancelNick"})
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal("HTTP cancellation did not stop lookup", err)
	}
	select {
	case <-upstreamStopped:
	case <-time.After(time.Second):
		t.Fatal("cancelled typing kept upstream work alive")
	}
	q := model.PeopleQuery{NetworkID: "one", Purpose: "mention", ConversationID: "channel", Query: "Peter"}
	page, err := m.LookupPeople(context.Background(), "caller", q)
	if err != nil || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
	w.mu.Lock()
	v := w.members["one:a"]
	v.Nick, v.DisplayName = "ChangedNick", "Changed display"
	w.members[v.ID] = v
	w.mu.Unlock()
	page, err = m.LookupPeople(context.Background(), "caller", q)
	if err != nil || len(page.Items) != 0 {
		t.Fatal("known people were served from stale result cache", page, err)
	}
}

func TestPeopleCacheBoundsExpiryAndCancelledInflight(t *testing.T) {
	var cache peopleCache
	for i := 0; i < 40; i++ {
		_, err := cache.lookup(context.Background(), string(rune('a'+i)), time.Minute, func(e *peopleCacheEntry) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(cache.entries) != 32 {
		t.Fatal("cache exceeded its bound", len(cache.entries))
	}
	var calls int
	load := func(e *peopleCacheEntry) error { calls++; return nil }
	_, _ = cache.lookup(context.Background(), "stale", -time.Second, load)
	_, _ = cache.lookup(context.Background(), "stale", time.Minute, load)
	if calls != 2 {
		t.Fatal("stale result was reused")
	}
	started, stopped := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = cache.lookup(ctx, "cancelled", time.Minute, func(e *peopleCacheEntry) error {
			close(started)
			<-ctx.Done()
			close(stopped)
			return ctx.Err()
		})
	}()
	<-started
	cancel()
	<-stopped
	wg.Wait()
	if cache.entries["cancelled"] != nil {
		t.Fatal("cancelled in-flight result remained cached")
	}
}
