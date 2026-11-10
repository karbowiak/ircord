package api

import (
	"encoding/json"
	"testing"

	"ircord/backend/internal/auth"
	"ircord/backend/internal/storage"
)

func TestDeviceNotificationAPIAccountScopeCASAndSafeMigration(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	laptop := browser(firefoxLinux)
	account := register(t, laptop, server.URL)
	phone := browser(safariIPhone)
	request(t, phone, "POST", server.URL+"/api/v1/auth/login", account.credentials(), 200)
	stranger := browser(firefoxLinux)
	register(t, stranger, server.URL)
	d := current(t, devices(t, phone, server.URL))
	p := storage.DevicePreferences{Content: "inherit", Delivery: "off", Sound: "mentions", Preview: true}
	request(t, stranger, "PUT", server.URL+"/api/v1/auth/devices/"+d.ID+"/notifications", p, 404)
	p.Migrate = true
	request(t, laptop, "PUT", server.URL+"/api/v1/auth/devices/"+d.ID+"/notifications", p, 404)
	p.Migrate = false
	p.Name = "Phone"
	var updated auth.Device
	if err := json.Unmarshal(request(t, laptop, "PUT", server.URL+"/api/v1/auth/devices/"+d.ID+"/notifications", p, 200), &updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Notifications.Explicit || updated.Notifications.Permission != "unknown" || updated.Notifications.Delivery != "off" {
		t.Fatal("remote setting granted a capability", updated)
	}
	request(t, laptop, "PUT", server.URL+"/api/v1/auth/devices/"+d.ID+"/notifications", p, 409)
	p.Migrate = true
	p.Delivery = "push"
	if err := json.Unmarshal(request(t, phone, "PUT", server.URL+"/api/v1/auth/devices/"+d.ID+"/notifications", p, 200), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Notifications.Delivery != "off" || !updated.Notifications.Explicit {
		t.Fatal("late local migration undid remote Off", updated)
	}
}
