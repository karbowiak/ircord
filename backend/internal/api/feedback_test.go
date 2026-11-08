package api

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/irc"
)

func TestAboutYouMultilineSurvivesLoginAndRestart(t *testing.T) {
	app, server, store, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	bio := "First line\nSecond line — hello 👋"
	var profile irc.Member
	json.Unmarshal(request(t, client, "PATCH", server.URL+"/api/v1/profile", map[string]any{"displayName": "Alice", "bio": bio}, 200), &profile)
	if profile.Bio != bio {
		t.Fatal("save response lost the bio")
	}
	request(t, client, "POST", server.URL+"/api/v1/auth/logout", nil, 204)
	request(t, client, "POST", server.URL+"/api/v1/auth/login", account.credentials(), 200)
	var snapshot irc.Snapshot
	json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/bootstrap", nil, 200), &snapshot)
	if snapshot.User.Bio != bio {
		t.Fatal("signing in lost the bio")
	}
	app.Manager.Close()
	restored := irc.New(irc.Options{Store: store, AllowPrivate: true})
	defer restored.Close()
	if restored.Snapshot(account.Account.ID).User.Bio != bio {
		t.Fatal("backend restart lost the bio")
	}
}

func TestUnavailableNetworkCanBeCanceledEditedAndRemoved(t *testing.T) {
	app, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	client.Timeout = 3 * time.Second
	account := register(t, client, server.URL)
	var network irc.Network
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/networks", map[string]any{
		"name": "Missing server", "host": "ircord-test.invalid", "port": 6697, "tls": true, "nick": "feedback",
	}, 200), &network)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		n := app.Manager.Snapshot(account.Account.ID).Networks[0]
		if n.Status == "error" && strings.Contains(strings.ToLower(n.StatusDetail), "address") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	n := app.Manager.Snapshot(account.Account.ID).Networks[0]
	if n.Status != "error" || n.StatusDetail == "" {
		t.Fatalf("unavailable network has no recoverable failure: %s %s", n.Status, n.StatusDetail)
	}
	url := server.URL + "/api/v1/networks/" + network.ID
	request(t, client, "PATCH", url, map[string]string{"status": "disconnected"}, 200)
	request(t, client, "PATCH", url, map[string]string{"host": "another-test.invalid"}, 200)
	request(t, client, "PATCH", url, map[string]string{"status": "connected"}, 200)
	request(t, client, "DELETE", url, nil, 204)
	if len(app.Manager.Snapshot(account.Account.ID).Networks) != 0 {
		t.Fatal("failed network could not be removed")
	}
}

func TestNetworkCheckRequiresAuthenticationAndDoesNotSaveNetwork(t *testing.T) {
	app, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	body := map[string]any{"host": "missing.invalid", "port": 6697, "tls": true}
	request(t, client, "POST", server.URL+"/api/v1/networks/check", body, 401)
	account := register(t, client, server.URL)
	var result NetworkCheck
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/networks/check", body, 200), &result)
	if result.Reachable || !strings.Contains(result.Detail, "address") {
		t.Fatalf("invalid host was verified: %+v", result)
	}
	if len(app.Manager.Snapshot(account.Account.ID).Networks) != 0 {
		t.Fatal("verification saved a network")
	}
}

func TestErgoFailedNetworkCanBeEditedToValidServer(t *testing.T) {
	address := os.Getenv("IRCORD_ERGO_ADDR")
	if address == "" {
		t.Skip("run make test-integration")
	}
	host, value, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(value)
	app, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	client.Timeout = 3 * time.Second
	account := register(t, client, server.URL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	var bad irc.Network
	json.Unmarshal(request(t, client, "POST", server.URL+"/api/v1/networks", map[string]any{
		"name": "Recover", "host": "feedback.invalid", "port": port, "tls": false, "nick": "fix" + suffix[len(suffix)-8:],
	}, 200), &bad)
	request(t, client, "PATCH", server.URL+"/api/v1/networks/"+bad.ID, map[string]any{"host": host, "port": port}, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.Manager.WaitConnected(ctx, account.Account.ID, bad.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Manager.Join(account.Account.ID, bad.ID, "#recover"+suffix); err != nil {
		t.Fatal(err)
	}
}
