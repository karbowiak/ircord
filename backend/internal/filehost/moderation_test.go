package filehost

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDeleteHostedURLDeletesOnlyDeletableUploads(t *testing.T) {
	store, dir, data := fixture(t)
	blob := Local{Root: filepath.Join(dir, "files")}
	service, err := New(Options{Store: store, Blob: blob, AvatarBlob: Local{Root: filepath.Join(dir, "avatars")}, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	upload := func(name string, provider Provider) string {
		t.Helper()
		u, err := service.Upload(ctx, "alice", "network", name, data, provider)
		if err != nil {
			t.Fatal(err)
		}
		attachment, err := service.Attachment("alice", "network", u.ID)
		if err != nil {
			t.Fatal(err)
		}
		return attachment.URL
	}
	hosted, avatar, upstream := upload("cat.png", service.LocalProvider()), upload("me.png", service.AvatarProvider()), upload("far.png", service.LocalProvider())
	if _, err = store.DB.Exec("UPDATE files SET backend='upstream' WHERE id=(SELECT m.id FROM file_metadata m WHERE json_extract(m.payload,'$.url')=?)", upstream); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"unknown": "https://elsewhere.example/cat.png", "avatar": avatar, "upstream": upstream} {
		if f, removed, err := service.DeleteHostedURL(ctx, value); removed || err != nil || f.ID != "" {
			t.Fatalf("%s: deleted %+v %v", name, f, err)
		}
	}
	unconfigured, err := New(Options{Store: store, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, removed, _ := unconfigured.DeleteHostedURL(ctx, hosted); removed {
		t.Fatal("deleted a file whose storage is not configured")
	}
	// Another account's upload: a server owner removes it and learns who uploaded it.
	f, removed, err := service.DeleteHostedURL(ctx, hosted)
	if err != nil || !removed || f.Owner != "alice" || f.ID == "" {
		t.Fatalf("hosted upload: %+v %v %v", f, removed, err)
	}
	if _, err = service.Record("alice", f.ID); err != ErrNotFound {
		t.Fatalf("the upload record remains: %v", err)
	}
	if _, removed, _ = service.DeleteHostedURL(ctx, hosted); removed {
		t.Fatal("deleted the same upload twice")
	}
}
