package api

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/admin"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/ergo"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

// saslLogin reports whether Ergo accepts a native SASL PLAIN login, like any IRC client.
func saslLogin(t *testing.T, addr, account, password string) bool {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	send := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	send("CAP REQ :sasl")
	send("NICK sasl" + strconv.FormatInt(time.Now().UnixNano()%1e8, 36))
	send("USER probe 0 * :probe")
	lines := bufio.NewScanner(conn)
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		switch {
		case len(fields) > 0 && fields[0] == "PING":
			send("PONG " + strings.Join(fields[1:], " "))
		case len(fields) > 3 && fields[1] == "CAP" && fields[3] == "ACK":
			send("AUTHENTICATE PLAIN")
		case len(fields) == 2 && fields[0] == "AUTHENTICATE" && fields[1] == "+":
			send("AUTHENTICATE " + base64.StdEncoding.EncodeToString([]byte(account+"\x00"+account+"\x00"+password)))
		case len(fields) > 1 && fields[1] == "903":
			send("QUIT")
			return true
		case len(fields) > 1 && (fields[1] == "904" || fields[1] == "902" || fields[1] == "908" || fields[1] == "CAP" && len(fields) > 3 && fields[3] == "NAK"):
			send("QUIT")
			return false
		}
	}
	t.Fatal("no SASL answer", lines.Err())
	return false
}

// TestErgoOperatorPasswordReset runs a reset through a real Ergo account API and checks the
// result the way IRC clients see it. The shared integration fixture has no HTTP API, so this
// needs an isolated Ergo with its API enabled:
//
//	IRCORD_ERGO_ADDR=127.0.0.1:26667 IRCORD_TEST_ERGO_API_URL=http://127.0.0.1:28089 \
//	IRCORD_TEST_ERGO_API_TOKEN=... go test -run TestErgoOperatorPasswordReset ./internal/api
func TestErgoOperatorPasswordReset(t *testing.T) {
	addr, apiURL, apiToken := os.Getenv("IRCORD_ERGO_ADDR"), os.Getenv("IRCORD_TEST_ERGO_API_URL"), os.Getenv("IRCORD_TEST_ERGO_API_TOKEN")
	if addr == "" || apiURL == "" || apiToken == "" {
		t.Skip("needs an isolated Ergo with its HTTP API: set IRCORD_ERGO_ADDR, IRCORD_TEST_ERGO_API_URL and IRCORD_TEST_ERGO_API_TOKEN")
	}
	upstream, err := ergo.New(ergo.Options{Server: addr, URL: apiURL, Token: apiToken, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	host, rawPort, _ := net.SplitHostPort(addr)
	port, _ := strconv.Atoi(rawPort)
	home := irc.NetworkConfig{ID: "ircord", Name: "IRCord", Host: host, Port: port}
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	options := irc.Options{Store: store, AllowPrivate: true}
	manager := irc.New(options)
	service := auth.New(store, auth.Options{Registration: true, Accounts: upstream,
		Provision: func(owner, username, password string) error {
			cfg := home
			cfg.Nick = username
			cfg.Auth = &model.AuthInput{Method: "sasl-plain", Account: username, Password: &password}
			return manager.ProvisionHome(owner, cfg)
		},
		PasswordChanged: func(owner, password string) error {
			var username string
			if err := store.DB.QueryRow("SELECT username FROM accounts WHERE id=?", owner).Scan(&username); err != nil {
				return err
			}
			return manager.UpdateHomePassword(owner, home, username, password)
		}})
	defer service.Close()
	token := strings.Repeat("ef", 32)
	verifier, _ := admin.NewVerifier(token)
	app := New(manager, Options{Store: store, Auth: service, Admin: verifier})
	server := httptest.NewServer(app.Handler)
	defer server.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano()%1e9, 36)
	target := testAccount{Username: "rst" + suffix, Password: "original-pass-1"}
	bystander := testAccount{Username: "kept" + suffix, Password: "unrelated-pass-1"}
	user := cookieClient()
	json.Unmarshal(request(t, user, "POST", server.URL+"/api/v1/auth/signup", target.credentials(), 201), &target.LoginResult)
	other := cookieClient()
	json.Unmarshal(request(t, other, "POST", server.URL+"/api/v1/auth/signup", bystander.credentials(), 201), &bystander.LoginResult)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := manager.WaitConnected(ctx, target.Account.ID, "ircord"); err != nil {
		t.Fatal(err)
	}
	if !saslLogin(t, addr, target.Username, target.Password) {
		t.Fatal("the original password is not an Ergo login")
	}

	secret := "operator-new-pass"
	code, body := adminPost(t, http.DefaultClient, server.URL+admin.Prefix+"/accounts/"+target.Username+"/password", token, map[string]string{"password": secret})
	var result auth.PasswordResetResult
	if code != 200 || json.Unmarshal(body, &result) != nil || result.Status != "completed" || !result.Associated || result.SessionsRevoked != 1 {
		t.Fatalf("reset: %d %s", code, body)
	}
	if saslLogin(t, addr, target.Username, target.Password) || !saslLogin(t, addr, target.Username, secret) {
		t.Fatal("Ergo did not move to the new password")
	}
	request(t, user, "GET", server.URL+"/api/v1/bootstrap", nil, 401)
	request(t, cookieClient(), "POST", server.URL+"/api/v1/auth/login", target.credentials(), 401)
	request(t, cookieClient(), "POST", server.URL+"/api/v1/auth/login", map[string]string{"username": target.Username, "password": secret}, 200)
	request(t, other, "GET", server.URL+"/api/v1/bootstrap", nil, 200)
	if !saslLogin(t, addr, bystander.Username, bystander.Password) {
		t.Fatal("an unrelated account's Ergo login changed")
	}

	// The saved home-network login signs in with the new password after a restart.
	manager.Close()
	restarted := irc.New(options)
	defer restarted.Close()
	if err := restarted.Restore(); err != nil {
		t.Fatal(err)
	}
	if err := restarted.WaitConnected(ctx, target.Account.ID, "ircord"); err != nil {
		t.Fatal(err)
	}
	for {
		if v, err := upstream.User(ctx, target.Username); err == nil && v.Account == target.Username {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("the restarted home connection is not signed in to its account")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// An account with no Ergo account fails the reset instead of locking it out.
	// It is created without association, like an account made before Ergo accounts were enabled.
	orphan := "orph" + suffix
	local := auth.New(store, auth.Options{Registration: true})
	defer local.Close()
	if _, err := local.Signup(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil), auth.SignupCredentials{Username: orphan, Password: "orphan-pass-1"}); err != nil {
		t.Fatal(err)
	}
	code, body = adminPost(t, http.DefaultClient, server.URL+admin.Prefix+"/accounts/"+orphan+"/password", token, map[string]string{"password": secret})
	if code != 409 || strings.Contains(string(body), secret) {
		t.Fatalf("reset of an account without an Ergo account: %d %s", code, body)
	}
	request(t, cookieClient(), "POST", server.URL+"/api/v1/auth/login", map[string]string{"username": orphan, "password": "orphan-pass-1"}, 200)
}
