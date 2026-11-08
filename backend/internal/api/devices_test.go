package api

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"ircord/backend/internal/auth"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/push"
)

type userAgentTransport string

func (ua userAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("User-Agent", string(ua))
	return http.DefaultTransport.RoundTrip(r)
}

// browser is a cookie jar (one browser profile) that identifies itself with a User-Agent.
func browser(ua string) *http.Client {
	client := cookieClient()
	client.Transport = userAgentTransport(ua)
	return client
}

func devices(t *testing.T, client *http.Client, base string) []auth.Device {
	t.Helper()
	var out auth.DeviceList
	if err := json.Unmarshal(request(t, client, "GET", base+"/api/v1/auth/devices", nil, 200), &out); err != nil {
		t.Fatal(err)
	}
	return out.Devices
}

func current(t *testing.T, list []auth.Device) auth.Device {
	t.Helper()
	for _, d := range list {
		if d.Current {
			return d
		}
	}
	t.Fatal("no device is marked current")
	return auth.Device{}
}

const firefoxLinux = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"
const safariIPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1"

func TestDevicesListLabelsAndIsolation(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	laptop := browser(firefoxLinux)
	account := register(t, laptop, server.URL)
	phone := browser(safariIPhone)
	request(t, phone, "POST", server.URL+"/api/v1/auth/login", account.credentials(), 200)
	stranger := browser(firefoxLinux)
	register(t, stranger, server.URL)

	list := devices(t, laptop, server.URL)
	if len(list) != 2 {
		t.Fatalf("want 2 devices, got %+v", list)
	}
	mine := current(t, list)
	if mine.Browser != "Firefox" || mine.OS != "Linux" || mine.LastSeenAt == nil || mine.Status != "recent" || mine.Push != "none" {
		t.Fatalf("current device metadata: %+v", mine)
	}
	other := list[0]
	if other.Current {
		other = list[1]
	}
	if other.Browser != "Safari" || other.OS != "iOS" {
		t.Fatalf("other device label: %+v", other)
	}
	// Tabs share the cookie, so a second request from the same profile is the same device.
	if again := current(t, devices(t, laptop, server.URL)); again.ID != mine.ID {
		t.Fatal("device ID is not stable across requests")
	}
	// The phone sees the same two devices with itself marked current.
	if fromPhone := current(t, devices(t, phone, server.URL)); fromPhone.ID != other.ID {
		t.Fatal("current-device detection differs between devices")
	}
	// Another account sees only its own device, and cannot sign out this account's.
	if theirs := devices(t, stranger, server.URL); len(theirs) != 1 || theirs[0].ID == mine.ID || theirs[0].ID == other.ID {
		t.Fatalf("cross-account device listing: %+v", theirs)
	}
	request(t, stranger, "DELETE", server.URL+"/api/v1/auth/devices/"+other.ID, nil, 404)
	request(t, phone, "GET", server.URL+"/api/v1/auth/session", nil, 200)
	if strings.Contains(string(request(t, phone, "GET", server.URL+"/api/v1/auth/session", nil, 200)), `"account":null`) {
		t.Fatal("another account signed this device out")
	}

	// No session IDs or token material reach the client.
	raw := string(request(t, laptop, "GET", server.URL+"/api/v1/auth/devices", nil, 200))
	rows, _ := store.DB.Query("SELECT id FROM sessions")
	for rows.Next() {
		var id string
		rows.Scan(&id)
		if strings.Contains(raw, id) {
			t.Fatal("a session ID leaked into the device list")
		}
	}
	rows.Close()
	for _, field := range []string{"token", "hash", "endpoint", "userAgent"} {
		if strings.Contains(raw, field) {
			t.Fatalf("device list exposes %q: %s", field, raw)
		}
	}

	// Sessions from before metadata was recorded are listed honestly, as unknown.
	store.DB.Exec("UPDATE sessions SET last_seen_ns=0, user_agent='' WHERE owner=? AND user_agent=?", account.Account.ID, firefoxLinux)
	for _, d := range devices(t, phone, server.URL) {
		if d.Current {
			continue
		}
		if d.Status != "unknown" || d.LastSeenAt != nil || d.Browser != "" || d.OS != "" {
			t.Fatalf("legacy session shown with invented metadata: %+v", d)
		}
	}
	// Expired and revoked sessions are not listed as authorized devices.
	store.DB.Exec("UPDATE sessions SET expires_ns=? WHERE owner=? AND user_agent=''", time.Now().Add(-time.Second).UnixNano(), account.Account.ID)
	if list := devices(t, phone, server.URL); len(list) != 1 || !list[0].Current {
		t.Fatalf("expired session listed: %+v", list)
	}
}

func TestSignOutRemoteDevice(t *testing.T) {
	var pushes *push.Service
	_, server, store, _ := authenticatedApp(t, func(options *irc.Options) {
		var err error
		pushes, err = push.New(push.Options{Store: options.Store, DataDir: t.TempDir(), Subject: "mailto:tests@example.invalid", Client: &http.Client{Transport: previewTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 201, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
		})}})
		if err != nil {
			t.Fatal(err)
		}
		options.Push = pushes
	})
	defer pushes.Close()
	laptop := browser(firefoxLinux)
	account := register(t, laptop, server.URL)
	phone := browser(safariIPhone)
	request(t, phone, "POST", server.URL+"/api/v1/auth/login", account.credentials(), 200)

	key, _ := ecdh.P256().GenerateKey(rand.Reader)
	secret := make([]byte, 16)
	rand.Read(secret)
	subscription := map[string]any{"endpoint": "https://fcm.googleapis.com/fcm/send/phone", "keys": map[string]string{"p256dh": base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), "auth": base64.RawURLEncoding.EncodeToString(secret)}}
	request(t, phone, "POST", server.URL+"/api/v1/push/subscriptions", map[string]any{"subscription": subscription, "device": "phone"}, 204)

	laptopSocket := accountSocket(t, laptop, server.URL, 0)
	phoneSocket := accountSocket(t, phone, server.URL, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var target auth.Device
	for _, d := range devices(t, laptop, server.URL) {
		if !d.Current {
			target = d
		} else if d.Status != "connected" {
			t.Fatalf("a device with a live connection is not connected: %+v", d)
		}
	}
	if target.Push != "registered" || target.Status != "connected" {
		t.Fatalf("phone status: %+v", target)
	}

	// Two tabs on the laptop sign the phone out at the same moment: one wins, one is told
	// the phone is already signed out, and neither fails.
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, _ := http.NewRequest("DELETE", server.URL+"/api/v1/auth/devices/"+target.ID, nil)
			r, err := laptop.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			r.Body.Close()
			codes[i] = r.StatusCode
		}(i)
	}
	wg.Wait()
	if !(codes[0] == 204 && codes[1] == 404 || codes[0] == 404 && codes[1] == 204) {
		t.Fatalf("concurrent sign-out statuses: %v", codes)
	}

	// The phone's socket closes and every later request, including a reconnect, is refused.
	phoneCtx, phoneDone := context.WithTimeout(context.Background(), 2*time.Second)
	defer phoneDone()
	for {
		_, raw, err := phoneSocket.Read(phoneCtx)
		if err != nil {
			// Closed by the server, not abandoned by our own deadline.
			if errors.Is(err, context.DeadlineExceeded) || phoneCtx.Err() != nil {
				t.Fatal("the signed-out device's socket stayed open")
			}
			break
		}
		if !strings.Contains(string(raw), "devices.updated") {
			t.Fatalf("unexpected event on the signed-out socket: %s", raw)
		}
	}
	request(t, phone, "GET", server.URL+"/api/v1/bootstrap", nil, 401)
	request(t, phone, "GET", server.URL+"/api/v1/auth/devices", nil, 401)
	dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dcancel()
	if _, resp, err := dialEvents(dctx, phone, server.URL); err == nil || resp == nil || resp.StatusCode != 401 {
		t.Fatal("a signed-out device reconnected")
	}
	var pushCount int
	store.DB.QueryRow("SELECT count(*) FROM push_subscriptions WHERE owner=?", account.Account.ID).Scan(&pushCount)
	if pushCount != 0 {
		t.Fatal("signed-out device kept its push subscription")
	}

	// The laptop stays signed in, connected, and is told the list changed.
	sawUpdate := false
	for !sawUpdate {
		_, raw, err := laptopSocket.Read(ctx)
		if err != nil {
			t.Fatal("the signing-out device lost its connection:", err)
		}
		sawUpdate = strings.Contains(string(raw), `"devices.updated"`)
	}
	if list := devices(t, laptop, server.URL); len(list) != 1 || !list[0].Current {
		t.Fatalf("after sign-out: %+v", list)
	}

	// Signing in again on the phone is a new device, not the revoked one coming back.
	request(t, phone, "POST", server.URL+"/api/v1/auth/login", account.credentials(), 200)
	for _, d := range devices(t, phone, server.URL) {
		if d.ID == target.ID {
			t.Fatal("a revoked session came back")
		}
	}

	// Signing out the current device clears its cookie.
	mine := current(t, devices(t, laptop, server.URL))
	req, _ := http.NewRequest("DELETE", server.URL+"/api/v1/auth/devices/"+mine.ID, nil)
	r, err := laptop.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != 204 || !strings.Contains(r.Header.Get("Set-Cookie"), "ircord_session=;") {
		t.Fatalf("current-device sign-out: %d %q", r.StatusCode, r.Header.Get("Set-Cookie"))
	}
	if strings.Contains(string(request(t, laptop, "GET", server.URL+"/api/v1/auth/session", nil, 200)), `"username"`) {
		t.Fatal("current device still signed in")
	}
	request(t, phone, "GET", server.URL+"/api/v1/bootstrap", nil, 200)
}

func TestSessionLastSeenIsThrottled(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	laptop := browser(firefoxLinux)
	register(t, laptop, server.URL)
	var first, second int64
	store.DB.QueryRow("SELECT last_seen_ns FROM sessions").Scan(&first)
	request(t, laptop, "GET", server.URL+"/api/v1/auth/devices", nil, 200)
	store.DB.QueryRow("SELECT last_seen_ns FROM sessions").Scan(&second)
	if first == 0 || second != first {
		t.Fatalf("last seen rewritten within a minute: %d then %d", first, second)
	}
	store.DB.Exec("UPDATE sessions SET last_seen_ns=?", time.Now().Add(-2*time.Minute).UnixNano())
	request(t, laptop, "GET", server.URL+"/api/v1/auth/devices", nil, 200)
	store.DB.QueryRow("SELECT last_seen_ns FROM sessions").Scan(&second)
	if time.Since(time.Unix(0, second)) > time.Minute {
		t.Fatal("last seen not refreshed after the interval")
	}
}

func TestDeviceLabel(t *testing.T) {
	for ua, want := range map[string][2]string{
		firefoxLinux: {"Firefox", "Linux"},
		safariIPhone: {"Safari", "iOS"},
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0": {"Edge", "Windows"},
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36":         {"Chrome", "macOS"},
		"Mozilla/5.0 (Linux; Android 15) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Mobile Safari/537.36":                  {"Chrome", "Android"},
		"Go-http-client/1.1": {"", ""},
		"":                   {"", ""},
	} {
		if b, o := auth.DeviceLabel(ua); b != want[0] || o != want[1] {
			t.Errorf("%q: got %q %q, want %q", ua, b, o, want)
		}
	}
}

func dialEvents(ctx context.Context, client *http.Client, base string) (*websocket.Conn, *http.Response, error) {
	return websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+"/api/v1/events", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{base}}})
}

// A session revoked behind the service's back (no watcher cancel) still loses its socket at
// the next heartbeat.
func TestHeartbeatClosesSocketsOfRevokedSessions(t *testing.T) {
	previous := heartbeatInterval
	heartbeatInterval = 100 * time.Millisecond
	t.Cleanup(func() { heartbeatInterval = previous })
	_, server, store, _ := authenticatedApp(t)
	laptop := browser(firefoxLinux)
	account := register(t, laptop, server.URL)
	socket := accountSocket(t, laptop, server.URL, 0)
	if _, err := store.DB.Exec("UPDATE sessions SET revoked=1 WHERE owner=?", account.Account.ID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		if _, _, err := socket.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatal("a revoked session's socket outlived the heartbeat")
			}
			return
		}
	}
}
