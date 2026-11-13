package api

import (
	"encoding/json"
	"io"
	"ircord/backend/internal/testsupport"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func TestPartyUploadsUseOrdinaryBearerURLsAfterLeaving(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	a := register(t, alice, server.URL)
	b := register(t, bob, server.URL)
	upload := offlinePNG(t, app, a.Account.ID)
	now := time.Now().UTC()
	p := storage.PartyRecord{Party: model.Party{ID: "party", ConversationID: "conversation", NetworkID: "home", OwnerID: a.Account.ID, State: "active", Revision: 1, CreatedAt: now, UpdatedAt: now, Members: []model.PartyMember{{PartyCandidate: model.PartyCandidate{AccountID: a.Account.ID, Username: a.Account.Username, DisplayName: a.Account.Username}, JoinedAt: now}}}, Channel: "#fixture", Creator: a.Account.ID, ClientID: "create", Fingerprint: "fixture"}
	testsupport.SeedLegacyParty(t, store, p)
	if _, _, e := store.BeginCommand(a.Account.ID, model.SendInput{ClientID: "private-upload", ConversationID: p.ConversationID, Text: upload.URL, Attachments: []string{upload.ID}}); e != nil {
		t.Fatal(e)
	}
	get := func(client *http.Client, want int) {
		t.Helper()
		response, e := client.Get(upload.URL)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("download status %d, want %d", response.StatusCode, want)
		}
	}
	get(http.DefaultClient, 200)
	get(bob, 200)
	get(alice, 200)
	if _, _, e := store.BeginCommand(a.Account.ID, model.SendInput{ClientID: "public-reuse", ConversationID: "ordinary", Text: upload.URL, Attachments: []string{upload.ID}}); e != nil {
		t.Fatal("ordinary bearer upload could not be reused", e)
	}
	p.Members = append(p.Members, model.PartyMember{PartyCandidate: model.PartyCandidate{AccountID: b.Account.ID, Username: b.Account.Username, DisplayName: b.Account.Username}, JoinedAt: now})
	p.Revision++
	if e := store.SaveParty(p, 1); e != nil {
		t.Fatal(e)
	}
	get(bob, 200)
	p.Members = p.Members[:1]
	p.Revision++
	if e := store.SaveParty(p, 2); e != nil {
		t.Fatal(e)
	}
	get(bob, 200)
	get(alice, 200)
}

type delayedPartyTransport struct{ started, release chan struct{} }

func (d delayedPartyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	close(d.started)
	select {
	case <-d.release:
	case <-r.Context().Done():
		return nil, r.Context().Err()
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"image/png"}}, Body: io.NopCloser(strings.NewReader("private bytes"))}, nil
}

func TestPartyOfflineDownloadRechecksMembershipAfterNetworkWait(t *testing.T) {
	app, server, s, _ := authenticatedApp(t)
	client := cookieClient()
	a := register(t, client, server.URL)
	owner := a.Account.ID
	now := time.Now()
	p := storage.PartyRecord{Party: model.Party{ID: "party", ConversationID: "private", NetworkID: "home", OwnerID: owner, State: "active", Revision: 1, Members: []model.PartyMember{{PartyCandidate: model.PartyCandidate{AccountID: owner, Username: a.Account.Username}, JoinedAt: now}}, CreatedAt: now, UpdatedAt: now}, Channel: "#private-fixture", Creator: owner, ClientID: "create"}
	testsupport.SeedLegacyParty(t, s, p)
	if _, e := s.Commit(owner, "conversation.updated", model.Conversation{ID: p.ConversationID, NetworkID: "home", Kind: "channel", Name: "Private", Party: &p.Party}); e != nil {
		t.Fatal(e)
	}
	remote := "https://example.com/private.png"
	if e := s.SaveHistory(owner, model.Message{ID: "message", ConversationID: p.ConversationID, SenderID: owner, Text: remote, Kind: "message", CreatedAt: now}); e != nil {
		t.Fatal(e)
	}
	delay := delayedPartyTransport{started: make(chan struct{}), release: make(chan struct{})}
	app.offlineHTTP = &http.Client{Transport: delay}
	status := make(chan int, 1)
	go func() {
		r, e := client.Get(server.URL + "/api/v1/offline/media?conversationId=private&messageId=message&url=" + url.QueryEscape(remote))
		if e != nil {
			status <- 0
			return
		}
		r.Body.Close()
		status <- r.StatusCode
	}()
	select {
	case <-delay.started:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not reach isolated transport")
	}
	p.Members = nil
	p.Revision++
	p.State = "archived"
	if e := s.SaveParty(p, 1); e != nil {
		t.Fatal(e)
	}
	close(delay.release)
	select {
	case got := <-status:
		if got != 404 {
			t.Fatal("delayed private bytes delivered after removal", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("download did not settle")
	}
}

func TestPartyUploadAllowsOrdinaryReuseAndChecksUploadOwner(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	alice := cookieClient()
	a := register(t, alice, server.URL)
	upload := offlinePNG(t, app, a.Account.ID)
	now := time.Now()
	p := storage.PartyRecord{Party: model.Party{ID: "party", ConversationID: "private", NetworkID: "home", OwnerID: a.Account.ID, State: "active", Revision: 1, Members: []model.PartyMember{{PartyCandidate: model.PartyCandidate{AccountID: a.Account.ID, Username: a.Account.Username, DisplayName: a.Account.Username}, JoinedAt: now}}, CreatedAt: now, UpdatedAt: now}, Channel: "#private-fixture", Creator: a.Account.ID, ClientID: "create"}
	testsupport.SeedLegacyParty(t, store, p)
	if _, _, e := store.BeginCommand(a.Account.ID, model.SendInput{ClientID: "public", ConversationID: "ordinary", Text: upload.URL, Attachments: []string{upload.ID}}); e != nil {
		t.Fatal(e)
	}
	if _, _, e := store.BeginCommand(a.Account.ID, model.SendInput{ClientID: "private", ConversationID: p.ConversationID, Text: upload.URL, Attachments: []string{upload.ID}}); e != nil {
		t.Fatal("ordinary file could not be shared in a party", e)
	}
	if _, _, e := store.BeginCommand("unrelated", model.SendInput{ClientID: "wrong-owner", ConversationID: p.ConversationID, Text: upload.URL, Attachments: []string{upload.ID}}); e == nil {
		t.Fatal("a bearer URL granted upload-owner mutation rights")
	}
	response, e := http.DefaultClient.Get(upload.URL)
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("failed private reuse changed ordinary file access", response.StatusCode)
	}
}

func TestPartyReplayRechecksMembershipWhenCleanupHasNotRun(t *testing.T) {
	_, server, s, _ := authenticatedApp(t)
	client := cookieClient()
	a := register(t, client, server.URL)
	owner := a.Account.ID
	now := time.Now()
	p := storage.PartyRecord{Party: model.Party{ID: "party", ConversationID: "private", NetworkID: "home", OwnerID: owner, State: "active", Revision: 1, Members: []model.PartyMember{{PartyCandidate: model.PartyCandidate{AccountID: owner, Username: a.Account.Username}, JoinedAt: now}}, CreatedAt: now, UpdatedAt: now}, Channel: "#private-fixture", Creator: owner, ClientID: "create"}
	testsupport.SeedLegacyParty(t, s, p)
	if _, e := s.Commit(owner, "message.created", model.Message{ID: "message", ConversationID: p.ConversationID, SenderID: owner, Text: "private replay bytes", Kind: "message", CreatedAt: now}); e != nil {
		t.Fatal(e)
	}
	var replay struct {
		Items []model.Event `json:"items"`
		Valid bool          `json:"valid"`
	}
	raw := request(t, client, "GET", server.URL+"/api/v1/events/replay?after=0", nil, 200)
	if e := json.Unmarshal(raw, &replay); e != nil || !replay.Valid || len(replay.Items) == 0 {
		t.Fatal("accepted account could not replay", e)
	}
	p.Members = nil
	p.State = "archived"
	p.Revision++
	if e := s.SaveParty(p, 1); e != nil {
		t.Fatal(e)
	}
	raw = request(t, client, "GET", server.URL+"/api/v1/events/replay?after=0", nil, 200)
	if e := json.Unmarshal(raw, &replay); e != nil || replay.Valid || len(replay.Items) != 0 || strings.Contains(string(raw), "private replay bytes") {
		t.Fatal("retained replay leaked after membership revocation", e)
	}
}
