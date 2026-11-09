package api

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"ircord/backend/internal/irc"
)

func TestTraceSocketSessionLifecycle(t *testing.T) {
	previous := heartbeatInterval
	heartbeatInterval = 50 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = previous })
	for _, end := range []string{"revoked", "expired", "logout"} {
		t.Run(end, func(t *testing.T) {
			app, server, store, _ := authenticatedApp(t)
			client := cookieClient()
			account := register(t, client, server.URL)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			host, rawPort, _ := net.SplitHostPort(listener.Addr().String())
			port, _ := strconv.Atoi(rawPort)
			if _, err := app.Manager.Connect(account.Account.ID, irc.NetworkConfig{ID: "trace", Name: "Trace", Host: host, Port: port, Nick: "trace"}); err != nil {
				t.Fatal(err)
			}
			endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/networks/trace/connection/trace/events"
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			socket, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{server.URL}}})
			if err != nil {
				t.Fatal(err)
			}
			defer socket.CloseNow()
			closed := make(chan error, 1)
			go func() {
				for {
					if _, _, err := socket.Read(ctx); err != nil {
						closed <- err
						return
					}
				}
			}()
			// A trace-only connection must refresh device presence just like events.
			old := time.Now().Add(-2 * time.Minute).UnixNano()
			if _, err := store.DB.Exec("UPDATE sessions SET last_seen_ns=? WHERE owner=?", old, account.Account.ID); err != nil {
				t.Fatal(err)
			}
			for {
				var seen int64
				if err := store.DB.QueryRow("SELECT last_seen_ns FROM sessions WHERE owner=?", account.Account.ID).Scan(&seen); err != nil {
					t.Fatal(err)
				}
				if seen > old {
					break
				}
				select {
				case err := <-closed:
					t.Fatalf("trace closed before refreshing presence: %v", err)
				case <-ctx.Done():
					t.Fatal("trace heartbeat did not refresh device presence")
				case <-time.After(10 * time.Millisecond):
				}
			}
			switch end {
			case "revoked":
				_, err = store.DB.Exec("UPDATE sessions SET revoked=1 WHERE owner=?", account.Account.ID)
			case "expired":
				_, err = store.DB.Exec("UPDATE sessions SET expires_ns=? WHERE owner=?", time.Now().Add(-time.Minute).UnixNano(), account.Account.ID)
			case "logout":
				request(t, client, http.MethodPost, server.URL+"/api/v1/auth/logout", nil, http.StatusNoContent)
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-closed:
				if ctx.Err() != nil {
					t.Fatalf("trace outlived its session: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("trace outlived its session")
			}
		})
	}
}

func TestTraceSocketRequiresOriginAndOwnNetwork(t *testing.T) {
	app, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	register(t, client, server.URL)
	other := cookieClient()
	otherAccount := register(t, other, server.URL)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	host, rawPort, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(rawPort)
	if _, err := app.Manager.Connect(otherAccount.Account.ID, irc.NetworkConfig{ID: "other", Name: "Other", Host: host, Port: port, Nick: "other"}); err != nil {
		t.Fatal(err)
	}
	request(t, client, http.MethodGet, server.URL+"/api/v1/networks/other/connection/trace/events", nil, http.StatusForbidden)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	socket, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/networks/other/connection/trace/events", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{server.URL}}})
	if err == nil {
		socket.CloseNow()
		t.Fatal("accepted another account's network")
	}
	if response == nil || response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected inaccessible network to return 404; got %v", response)
	}
}
