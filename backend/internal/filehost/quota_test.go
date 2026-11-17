package filehost

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func quotaService(t *testing.T, options Options) (*Service, []byte) {
	t.Helper()
	store, dir, data := fixture(t)
	options.Store = store
	if options.Blob == nil {
		options.Blob = Local{Root: filepath.Join(dir, "files")}
	}
	options.AvatarBlob = Local{Root: filepath.Join(dir, "avatars")}
	options.PublicURL = "https://chat.example"
	service, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	return service, data
}

func TestUploadQuotaAllowsExactlyTheLimit(t *testing.T) {
	ctx := context.Background()
	t.Run("files", func(t *testing.T) {
		service, data := quotaService(t, Options{QuotaFiles: 2})
		first, err := service.Upload(ctx, "alice", "n", "1.png", data, service.LocalProvider())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = service.Upload(ctx, "alice", "n", "2.png", data, service.LocalProvider()); err != nil {
			t.Fatal("the last file within the limit was refused:", err)
		}
		_, err = service.Upload(ctx, "alice", "n", "3.png", data, service.LocalProvider())
		if !errors.Is(err, ErrQuota) || !strings.Contains(err.Error(), "limit of 2 uploaded files") {
			t.Fatal("file past the limit:", err)
		}
		if files(t, service, "alice") != 2 {
			t.Fatal("refused upload was stored")
		}
		// Other accounts and profile images have their own limits.
		if _, err = service.Upload(ctx, "bob", "n", "1.png", data, service.LocalProvider()); err != nil {
			t.Fatal(err)
		}
		if _, err = service.Upload(ctx, "alice", "", "avatar", data, service.AvatarProvider()); err != nil {
			t.Fatal("profile image counted toward the upload quota:", err)
		}
		if err = service.Delete(ctx, "alice", first.ID); err != nil {
			t.Fatal(err)
		}
		if _, err = service.Upload(ctx, "alice", "n", "3.png", data, service.LocalProvider()); err != nil {
			t.Fatal("deleting a file did not free its place:", err)
		}
	})
	t.Run("bytes", func(t *testing.T) {
		service, data := quotaService(t, Options{})
		service.options.QuotaBytes = int64(2*len(data) + 1)
		for _, name := range []string{"1.png", "2.png"} {
			if _, err := service.Upload(ctx, "alice", "n", name, data, service.LocalProvider()); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := service.Upload(ctx, "alice", "n", "3.png", data, service.LocalProvider()); !errors.Is(err, ErrQuota) {
			t.Fatal("bytes past the limit:", err)
		}
		entries, _ := os.ReadDir(service.options.Blob.(Local).Root)
		if len(entries) != 2 {
			t.Fatal("refused upload left its object in storage", len(entries))
		}
		service.options.QuotaBytes = int64(3 * len(data))
		if _, err := service.Upload(ctx, "alice", "n", "3.png", data, service.LocalProvider()); err != nil {
			t.Fatal("upload that exactly fills the quota was refused:", err)
		}
	})
}

func TestUploadQuotaHoldsUnderConcurrentUploads(t *testing.T) {
	service, data := quotaService(t, Options{QuotaFiles: 3})
	var wg sync.WaitGroup
	var mu sync.Mutex
	stored := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := service.Upload(context.Background(), "alice", "n", "x.png", data, service.LocalProvider()); err == nil {
				mu.Lock()
				stored++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if stored != 3 || files(t, service, "alice") != 3 {
		t.Fatal("concurrent uploads passed the quota", stored, files(t, service, "alice"))
	}
}

// age moves an upload's creation time back, past the sweep's grace period.
func age(t *testing.T, service *Service, id string, by time.Duration) {
	t.Helper()
	if _, err := service.options.Store.DB.Exec("UPDATE files SET created_ns=created_ns-? WHERE id=?", by.Nanoseconds(), id); err != nil {
		t.Fatal(err)
	}
}

func TestSweepRemovesOnlyUploadsNothingUses(t *testing.T) {
	ctx := context.Background()
	service, data := quotaService(t, Options{})
	db := service.options.Store.DB
	upload := func(name string) Upload {
		t.Helper()
		u, err := service.Upload(ctx, "alice", "n", name, data, service.LocalProvider())
		if err != nil {
			t.Fatal(err)
		}
		age(t, service, u.ID, 48*time.Hour)
		return u
	}
	unused := upload("unused.png")
	sent := upload("sent.png")
	if _, err := service.Attachment("alice", "n", sent.ID); err != nil {
		t.Fatal(err)
	}
	pasted := upload("pasted.png")
	if _, err := db.Exec("INSERT INTO messages(owner,conversation,id,upstream,created_ns,text,sender,kind,payload) VALUES('alice','c','m1','',1,?,'alice','message','{}')", "look "+pasted.URL); err != nil {
		t.Fatal(err)
	}
	banner := upload("banner.png")
	if _, err := db.Exec("INSERT INTO entities(owner,kind,id,payload) VALUES('alice','network','n',?)", `{"profile":{"bannerUrl":"`+banner.URL+`"}}`); err != nil {
		t.Fatal(err)
	}
	fresh, err := service.Upload(ctx, "alice", "n", "fresh.png", data, service.LocalProvider())
	if err != nil {
		t.Fatal(err)
	}
	avatar, err := service.Upload(ctx, "alice", "", "avatar", data, service.AvatarProvider())
	if err != nil {
		t.Fatal(err)
	}
	age(t, service, avatar.ID, 48*time.Hour)

	removed, err := service.SweepUnused(ctx, 24*time.Hour)
	if err != nil || removed != 1 {
		t.Fatal("first sweep", removed, err)
	}
	if _, err = service.Record("alice", unused.ID); err != ErrNotFound {
		t.Fatal("unused upload kept")
	}
	for name, id := range map[string]string{"sent": sent.ID, "pasted": pasted.ID, "banner": banner.ID, "fresh": fresh.ID, "avatar": avatar.ID} {
		if _, err = service.Record("alice", id); err != nil {
			t.Fatalf("%s upload was removed", name)
		}
	}
	// A message match is permanent; a profile match lasts only while the profile uses it.
	var marked int
	_ = db.QueryRow("SELECT count(*) FROM file_uses WHERE id IN (?,?)", pasted.ID, banner.ID).Scan(&marked)
	if marked != 1 {
		t.Fatal("unexpected use records", marked)
	}
	removed, err = service.SweepUnused(ctx, 24*time.Hour)
	if err != nil || removed != 0 {
		t.Fatal("second sweep changed something", removed, err)
	}
	if _, err = db.Exec("DELETE FROM entities WHERE owner='alice' AND kind='network'"); err != nil {
		t.Fatal(err)
	}
	if removed, err = service.SweepUnused(ctx, 24*time.Hour); err != nil || removed != 1 {
		t.Fatal("replaced banner kept", removed, err)
	}
	if removed, err = service.SweepUnused(ctx, 0); err != nil || removed != 0 {
		t.Fatal("a zero grace period must keep every upload", removed, err)
	}
}

func TestSweepContinuesPastAFailingFileAndRetriesIt(t *testing.T) {
	ctx := context.Background()
	store, dir, data := fixture(t)
	root := filepath.Join(dir, "files")
	service, err := New(Options{Store: store, Blob: Local{Root: root}, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, name := range []string{"one.png", "two.png", "three.png"} {
		u, err := service.Upload(ctx, "alice", "n", name, data, service.LocalProvider())
		if err != nil {
			t.Fatal(err)
		}
		age(t, service, u.ID, 48*time.Hour)
		ids = append(ids, u.ID)
	}
	service.options.Blob = failingBlob{Local: Local{Root: root}, key: ids[1]}
	removed, err := service.SweepUnused(ctx, 24*time.Hour)
	if err == nil || removed != 2 {
		t.Fatal("one failing file must not stop the others", removed, err)
	}
	// The failed file can no longer be sent or downloaded, but its record stays for the retry.
	if _, err = service.Attachment("alice", "n", ids[1]); err != ErrNotFound {
		t.Fatal("a file being removed can still be sent")
	}
	if files(t, service, "alice") != 1 {
		t.Fatal("record of the failed file lost")
	}
	service.options.Blob = Local{Root: root}
	if removed, err = service.SweepUnused(ctx, 24*time.Hour); err != nil || removed != 1 {
		t.Fatal("retry", removed, err)
	}
	if files(t, service, "alice") != 0 {
		t.Fatal("file left after retry")
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatal("objects left in storage", len(entries))
	}
}

func TestSweepKeepsFilesFromBeforeTracking(t *testing.T) {
	ctx := context.Background()
	service, data := quotaService(t, Options{})
	old, err := service.Upload(ctx, "alice", "n", "old.png", data, service.LocalProvider())
	if err != nil {
		t.Fatal(err)
	}
	age(t, service, old.ID, 48*time.Hour)
	// An instance upgraded from a version without use tracking.
	if _, err = service.options.Store.DB.Exec("DROP TABLE file_uses"); err != nil {
		t.Fatal(err)
	}
	if service, err = New(service.options); err != nil {
		t.Fatal(err)
	}
	if removed, err := service.SweepUnused(ctx, 24*time.Hour); err != nil || removed != 0 {
		t.Fatal("upload from before tracking was removed", removed, err)
	}
	// Opening again does not mark later uploads.
	unused, err := service.Upload(ctx, "alice", "n", "new.png", data, service.LocalProvider())
	if err != nil {
		t.Fatal(err)
	}
	age(t, service, unused.ID, 48*time.Hour)
	if service, err = New(service.options); err != nil {
		t.Fatal(err)
	}
	if removed, err := service.SweepUnused(ctx, 24*time.Hour); err != nil || removed != 1 {
		t.Fatal("new unused upload", removed, err)
	}
}

func TestDeleteAvatarURLDeletesOnlyTheOwnersProfileImage(t *testing.T) {
	ctx := context.Background()
	service, data := quotaService(t, Options{})
	avatar, err := service.Upload(ctx, "alice", "", "avatar", data, service.AvatarProvider())
	if err != nil {
		t.Fatal(err)
	}
	regular, err := service.Upload(ctx, "alice", "n", "banner.png", data, service.LocalProvider())
	if err != nil {
		t.Fatal(err)
	}
	for _, try := range []struct{ owner, url string }{{"bob", avatar.URL}, {"alice", regular.URL}, {"alice", "https://elsewhere.example/a.png"}} {
		if err = service.DeleteAvatarURL(ctx, try.owner, try.url); err != nil {
			t.Fatal(err)
		}
	}
	if files(t, service, "alice") != 2 {
		t.Fatal("deleted a file that is not the owner's profile image")
	}
	if err = service.DeleteAvatarURL(ctx, "alice", avatar.URL); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Record("alice", avatar.ID); err != ErrNotFound {
		t.Fatal("profile image kept")
	}
}

func TestDeleteAccountRemovesFilesASweepLeftHidden(t *testing.T) {
	ctx := context.Background()
	service, data := quotaService(t, Options{})
	u, err := service.Upload(ctx, "alice", "n", "x.png", data, service.LocalProvider())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.options.Store.DB.Exec("UPDATE files SET deleted=1 WHERE id=?", u.ID); err != nil {
		t.Fatal(err)
	}
	if err = service.DeleteAccount(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if files(t, service, "alice") != 0 {
		t.Fatal("hidden file kept")
	}
}
