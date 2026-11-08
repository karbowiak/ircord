package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"ircord/backend/internal/activity"
	"ircord/backend/internal/irc"
)

// tabSocket opens an event connection for one tab and collects its notification alerts.
func tabSocket(t *testing.T, client *http.Client, base, tab string) (ready map[string]any, alerts <-chan activity.Alert) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(base, "http")+"/api/v1/events", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{base}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { socket.CloseNow() })
	_, raw, err := socket.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(raw, &ready)
	resume, _ := json.Marshal(map[string]any{"v": 1, "type": "session.resume", "after": 0, "tab": tab})
	if err = socket.Write(ctx, websocket.MessageText, resume); err != nil {
		t.Fatal(err)
	}
	out := make(chan activity.Alert, 16)
	go func() {
		for {
			_, raw, err := socket.Read(ctx)
			if err != nil {
				return
			}
			var e struct {
				Type string         `json:"type"`
				Data activity.Alert `json:"data"`
			}
			if json.Unmarshal(raw, &e) == nil && e.Type == "notification.alert" {
				out <- e.Data
			}
		}
	}()
	return ready, out
}

func noAlert(t *testing.T, name string, alerts <-chan activity.Alert) {
	t.Helper()
	select {
	case a := <-alerts:
		t.Fatalf("%s received an alert it should not: %+v", name, a)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestActivityRoutesAlertsToOneTabOfTheRightDevice(t *testing.T) {
	router := activity.New(activity.Options{})
	_, server, _, _ := authenticatedApp(t, func(o *irc.Options) { o.Router = router })
	laptop := browser(firefoxLinux)
	account := register(t, laptop, server.URL)
	phone := browser(safariIPhone)
	request(t, phone, "POST", server.URL+"/api/v1/auth/login", account.credentials(), 200)
	stranger := browser(firefoxLinux)
	other := register(t, stranger, server.URL)

	ready, first := tabSocket(t, laptop, server.URL, "laptop-1")
	if ready["routing"] != true {
		t.Fatalf("session.ready should announce server routing: %v", ready)
	}
	_, second := tabSocket(t, laptop, server.URL, "laptop-2")
	_, onPhone := tabSocket(t, phone, server.URL, "phone-1")
	_, theirs := tabSocket(t, stranger, server.URL, "stranger-1")

	request(t, cookieClient(), "POST", server.URL+"/api/v1/activity", map[string]any{"tab": "x", "active": true}, 401)
	request(t, laptop, "POST", server.URL+"/api/v1/activity", map[string]any{"tab": "", "active": true}, 422)
	request(t, laptop, "POST", server.URL+"/api/v1/activity", map[string]any{"tab": "laptop-2", "active": true, "formFactor": "watch"}, 422)
	request(t, laptop, "POST", server.URL+"/api/v1/activity", map[string]any{"tab": "laptop-2", "active": true, "formFactor": "desktop", "canAlert": true}, 204)
	// Another account's activity does not quiet this one, nor the other way round.
	request(t, stranger, "POST", server.URL+"/api/v1/activity", map[string]any{"tab": "stranger-1", "active": true}, 204)
	if !router.Active(account.Account.ID) || !router.Active(other.Account.ID) {
		t.Fatal("reports should hold activity leases")
	}

	item := activity.Item{Conversation: "dm", Message: "m1", At: time.Now(), Title: "bob", Body: "hello", Kind: "mention"}
	delays := activity.Delays{Target: 0, All: time.Hour}
	router.Notify(account.Account.ID, item, delays)
	noAlert(t, "an active account", first)

	request(t, laptop, "POST", server.URL+"/api/v1/activity", map[string]any{"tab": "laptop-2", "active": false, "canAlert": true}, 204)
	item.Message = "m2"
	router.Notify(account.Account.ID, item, delays)
	select {
	case a := <-second:
		if a.MessageID != "m2" || a.Title != "bob" || a.Kind != "mention" || a.Unread != 1 {
			t.Fatalf("unexpected alert %+v", a)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the most recently active tab of the last-used device was not alerted")
	}
	noAlert(t, "the laptop's other tab", first)
	noAlert(t, "the phone, during the last-device window", onPhone)
	noAlert(t, "another account", theirs)

	// Signing out ends the session's lease at once, without waiting for it to lapse.
	request(t, laptop, "POST", server.URL+"/api/v1/activity", map[string]any{"tab": "laptop-2", "active": true, "canAlert": true}, 204)
	if !router.Active(account.Account.ID) {
		t.Fatal("the laptop should be active again")
	}
	request(t, laptop, "POST", server.URL+"/api/v1/auth/logout", nil, 204)
	if router.Active(account.Account.ID) {
		t.Fatal("a signed-out session must stop holding alerts back")
	}
}

func TestNotificationDelaySettingsValidateAndDefault(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	register(t, client, server.URL)
	var settings map[string]any
	if err := json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/notifications/settings", nil, 200), &settings); err != nil {
		t.Fatal(err)
	}
	if settings["lastDeviceDelay"] != float64(30) || settings["allDevicesDelay"] != float64(300) || settings["autoAwayDelay"] != float64(600) {
		t.Fatalf("defaults: %v", settings)
	}
	settings["lastDeviceDelay"], settings["allDevicesDelay"] = 120, 60
	request(t, client, "PUT", server.URL+"/api/v1/notifications/settings", settings, 422)
	settings["lastDeviceDelay"], settings["allDevicesDelay"] = -1, 60
	request(t, client, "PUT", server.URL+"/api/v1/notifications/settings", settings, 422)
	settings["lastDeviceDelay"], settings["allDevicesDelay"], settings["autoAwayDelay"] = 15, 120, 0
	request(t, client, "PUT", server.URL+"/api/v1/notifications/settings", settings, 200)
	var saved map[string]any
	_ = json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/notifications/settings", nil, 200), &saved)
	if saved["lastDeviceDelay"] != float64(15) || saved["allDevicesDelay"] != float64(120) || saved["autoAwayDelay"] != float64(0) {
		t.Fatalf("saved: %v", saved)
	}
}
