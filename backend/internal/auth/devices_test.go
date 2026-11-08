package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

func signedIn(t *testing.T, service *Service, ua string) Principal {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://localhost/auth/signup", nil)
	r.Header.Set("User-Agent", ua)
	if _, err := service.Signup(w, r, SignupCredentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "http://localhost/api/v1/bootstrap", nil)
	req.AddCookie(w.Result().Cookies()[0])
	p, err := service.Authenticate(req)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A revocation that commits but then reports an error (a failed post-commit step) must still
// cut the session's live connections; one that did not commit must leave them alone.
func TestRevokeCutsConnectionsWhateverTheRevokeReports(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := New(store, Options{Registration: true})
	p := signedIn(t, service, "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0")

	ctx, unwatch, err := service.Watch(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer unwatch()
	failed := errors.New("cleanup failed")
	// Nothing was revoked: the error comes back and the socket stays.
	if err = service.revoke(p.Account.ID, p.Session.ID, func(string, string) error { return failed }); !errors.Is(err, failed) {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal("an unrevoked session lost its socket")
	}
	// Revoked, then an error: the error still comes back, but the socket is cut.
	err = service.revoke(p.Account.ID, p.Session.ID, func(owner, id string) error {
		if err := store.RevokeSession(owner, id); err != nil {
			t.Fatal(err)
		}
		return failed
	})
	if !errors.Is(err, failed) {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("a revoked session kept its socket after a cleanup error")
	}
}

func TestSessionMetadataIsWrittenWithTheSession(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := New(store, Options{Registration: true})
	p := signedIn(t, service, "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1")
	if p.Session.LastSeenAt.IsZero() {
		t.Fatal("a new session has no last-seen time")
	}
	list, err := service.Devices(p)
	if err != nil || len(list.Devices) != 1 || list.Devices[0].Browser != "Safari" || list.Devices[0].OS != "iOS" {
		t.Fatalf("%+v %v", list, err)
	}
}
