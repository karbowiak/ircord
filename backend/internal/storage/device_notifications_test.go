package storage

import (
	"errors"
	"fmt"
	"ircord/backend/internal/model"
	"testing"
	"time"
)

func TestDeviceNotificationMigrationCASCapabilitiesAndRestart(t *testing.T) {
	dir := t.TempDir()
	s := deviceStore(t, dir)
	if _, err := s.CreateUser("owner", "owner", "hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateVerifiedSession("phone", "owner", []byte("phone"), time.Now().Add(time.Hour), "hash", "phone"); err != nil {
		t.Fatal(err)
	}
	d, err := s.NotificationDevice("owner", "phone")
	if err != nil {
		t.Fatal(err)
	}
	if d.Notifications.Delivery != "off" || d.Notifications.Content != "inherit" || d.Notifications.Initialized || !d.Notifications.Preview {
		t.Fatal("unsafe migration default", d)
	}
	p := DevicePreferences{Name: "Phone", Content: "inherit", Delivery: "push", Sound: "all", Preview: true, Migrate: true}
	if err = s.SetDevicePreferences("owner", "phone", p); err != nil {
		t.Fatal(err)
	}
	d, _ = s.NotificationDevice("owner", "phone")
	if !d.Notifications.Initialized || d.Notifications.Explicit || d.Notifications.Revision != 1 {
		t.Fatal("migration", d)
	}
	p.Migrate = false
	p.Revision = 1
	p.Delivery = "off"
	if err = s.SetDevicePreferences("owner", "phone", p); err != nil {
		t.Fatal(err)
	}
	if err = s.SetDevicePreferences("owner", "phone", p); !errors.Is(err, ErrDevicePreferenceConflict) {
		t.Fatal("stale full write accepted", err)
	}
	p.Migrate = true
	p.Delivery = "push"
	if err = s.SetDevicePreferences("owner", "phone", p); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	if err = s.ReportDeviceCapabilities("owner", "phone", "granted", "phone", true, at); err != nil {
		t.Fatal(err)
	}
	if err = s.ReportDeviceCapabilities("owner", "phone", "denied", "desktop", false, at.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s = deviceStore(t, dir)
	defer s.Close()
	d, err = s.NotificationDevice("owner", "phone")
	if err != nil {
		t.Fatal(err)
	}
	if d.Notifications.Delivery != "off" || !d.Notifications.Explicit || d.Notifications.Revision != 2 || d.Notifications.Permission != "granted" || d.FormFactor != "phone" || d.LastActiveAt == nil || !d.LastActiveAt.Equal(at) {
		t.Fatal("remote Off, capability ordering or persistence", d)
	}
	if err = s.SetDevicePreferences("other", "phone", p); err != nil && !errors.Is(err, ErrDevicePreferenceConflict) {
		t.Fatal(err)
	}
	unchanged, _ := s.NotificationDevice("owner", "phone")
	if unchanged.Notifications.Delivery != "off" {
		t.Fatal("another account changed preferences")
	}
}

func TestNotificationUnreadUsesDurableReadDeletionAndAccountScope(t *testing.T) {
	s := deviceStore(t, t.TempDir())
	defer s.Close()
	account(t, s, "alice")
	account(t, s, "bob")
	at := time.Now().UTC().Truncate(time.Millisecond)
	for _, owner := range []string{"alice", "bob"} {
		for i := 0; i < 3; i++ {
			msg := model.Message{ID: fmt.Sprint("m", i), ConversationID: "room", SenderID: "peer", Kind: "message", CreatedAt: at.Add(time.Duration(i) * time.Second)}
			if err := s.SaveHistory(owner, msg); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.MarkRead("alice", "room", at.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if unread, err := s.NotificationMessageUnread("alice", "room", "m1"); err != nil || unread {
		t.Fatal("read message", unread, err)
	}
	if unread, err := s.NotificationMessageUnread("bob", "room", "m1"); err != nil || !unread {
		t.Fatal("cross-account read", unread, err)
	}
	if _, err := s.MarkUnread("alice", "room", "m1"); err != nil {
		t.Fatal(err)
	}
	if unread, err := s.NotificationMessageUnread("alice", "room", "m1"); err != nil || !unread {
		t.Fatal("manual unread boundary", unread, err)
	}
	if _, err := s.DB.Exec(`UPDATE messages SET payload=json_set(payload,'$.deleted',json('true')) WHERE owner='alice' AND id='m1'`); err != nil {
		t.Fatal(err)
	}
	if unread, err := s.NotificationMessageUnread("alice", "room", "m1"); err != nil || unread {
		t.Fatal("deleted message", unread, err)
	}
}

func TestDeviceNotificationForwardMigrationPreservesPreferences(t *testing.T) {
	dir := t.TempDir()
	s := deviceStore(t, dir)
	if _, err := s.CreateUser("owner", "owner", "hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateVerifiedSession("phone", "owner", []byte("phone"), time.Now().Add(time.Hour), "hash", "phone"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDevicePreferences("owner", "phone", DevicePreferences{Name: "Phone", Content: "mentions", Delivery: "off", Sound: "all", Preview: false}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{`ALTER TABLE session_notification_settings DROP COLUMN revision`, `UPDATE session_notification_settings SET preferences=json_remove(preferences,'$.explicit')`, `DELETE FROM goose_db_version WHERE version_id=30`} {
		if _, err := s.DB.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	s = deviceStore(t, dir)
	defer s.Close()
	d, err := s.NotificationDevice("owner", "phone")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Phone" || d.Notifications.Content != "mentions" || d.Notifications.Delivery != "off" || d.Notifications.Sound != "all" || d.Notifications.Preview || !d.Notifications.Explicit {
		t.Fatal("forward migration changed desired settings", d)
	}
}
