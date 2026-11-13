package filehost

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// failingBlob is local storage that can't delete one object.
type failingBlob struct {
	Local
	key string
}

func (b failingBlob) Delete(ctx context.Context, key string) error {
	if key == b.key {
		return errors.New("storage unreachable")
	}
	return b.Local.Delete(ctx, key)
}

func files(t *testing.T, service *Service, owner string) int {
	t.Helper()
	var n int
	if err := service.options.Store.DB.QueryRow("SELECT count(*) FROM files WHERE owner=?", owner).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Deleting an account's files must not stop at one file that fails, and must keep the record
// of a file whose storage is not configured until it can be removed.
func TestDeleteAccountFinishesWhatItCan(t *testing.T) {
	store, dir, data := fixture(t)
	blob := Local{Root: filepath.Join(dir, "files")}
	service, err := New(Options{Store: store, Blob: blob, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ids := []string{}
	for _, name := range []string{"one.png", "two.png", "three.png"} {
		upload, err := service.Upload(ctx, "alice", "network", name, data, service.LocalProvider())
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, upload.ID)
	}
	// The middle file fails, so in either visiting order another file comes after it.
	record, err := service.Record("alice", ids[1])
	if err != nil {
		t.Fatal(err)
	}
	failing, err := New(Options{Store: store, Blob: failingBlob{blob, record.ObjectKey}, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	if err = failing.DeleteAccount(ctx, "alice"); err == nil {
		t.Fatal("a failed file deletion was not reported")
	}
	if n := files(t, service, "alice"); n != 1 {
		t.Fatal("one failed file stopped the others, or was forgotten:", n)
	}

	// While its storage is not configured, the file's record stays and deletion reports it.
	unconfigured, err := New(Options{Store: store, PublicURL: "https://chat.example"})
	if err != nil {
		t.Fatal(err)
	}
	if err = unconfigured.DeleteAccount(ctx, "alice"); err == nil {
		t.Fatal("a file in unconfigured storage was reported as deleted")
	}
	if n := files(t, service, "alice"); n != 1 {
		t.Fatal("the record of a file in unconfigured storage was dropped:", n)
	}
	// Once the storage is back, the next attempt removes it.
	if err = service.DeleteAccount(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	if n := files(t, service, "alice"); n != 0 {
		t.Fatal("files left behind:", n)
	}
}
