package filehost

import (
	"context"
	"net/url"
	"testing"
	"time"
)

// A theme stores its background as the upload's path on this instance. That
// reference in the account's themes document keeps the image through the
// cleanup, and removing the theme lets the cleanup take it.
func TestSweepKeepsThemeBackgrounds(t *testing.T) {
	ctx := context.Background()
	service, data := quotaService(t, Options{})
	db := service.options.Store.DB
	background, err := service.Upload(ctx, "alice", "n", "background.png", data, service.LocalProvider())
	if err != nil {
		t.Fatal(err)
	}
	age(t, service, background.ID, 48*time.Hour)
	address, err := url.Parse(background.URL)
	if err != nil {
		t.Fatal(err)
	}
	document := `{"format":1,"themes":[{"id":"t1","background":{"url":"` + address.Path + `"}}]}`
	if _, err = db.Exec("INSERT INTO app_data(owner,kind,payload,updated_ns) VALUES('alice','themes.updated',?,1)", document); err != nil {
		t.Fatal(err)
	}
	if removed, err := service.SweepUnused(ctx, 24*time.Hour); err != nil || removed != 0 {
		t.Fatal("theme background removed", removed, err)
	}
	if _, err = db.Exec("UPDATE app_data SET payload='{\"format\":1,\"themes\":[]}' WHERE owner='alice' AND kind='themes.updated'"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE file_uses SET expires_ns=1 WHERE expires_ns!=0"); err != nil {
		t.Fatal(err)
	}
	if removed, err := service.SweepUnused(ctx, 24*time.Hour); err != nil || removed != 1 {
		t.Fatal("background of a deleted theme kept", removed, err)
	}
}
