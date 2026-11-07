package auth

import (
	"context"
	"errors"
	"fmt"
	"ircord/backend/internal/storage"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestSessionsPersistRevokeAndIsolate(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := New(store, Options{Registration: true})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "http://localhost/auth/register", nil)
	principal, err := service.Login(w, r, Credentials{}, true)
	if err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("missing secure session boundary")
	}
	authenticated := httptest.NewRequest("GET", "http://localhost/api/v1/bootstrap", nil)
	authenticated.AddCookie(cookies[0])
	p, err := service.Authenticate(authenticated)
	if err != nil || p.Account.ID != principal.Account.ID {
		t.Fatal(p, err)
	}
	ctx, unwatch, err := service.Watch(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer unwatch()
	if err = service.Revoke("another-owner", p.Session.ID); err == nil {
		t.Fatal("cross-account revocation accepted")
	}
	if err = service.Revoke(p.Account.ID, p.Session.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("revoked session kept its socket")
	}
	if _, err = service.Authenticate(authenticated); err == nil {
		t.Fatal("revoked cookie accepted")
	}
	w = httptest.NewRecorder()
	login, loginErr := service.Login(w, r, Credentials{Key: principal.Key}, false)
	if loginErr != nil {
		t.Fatal(loginErr)
	}
	cookie := w.Result().Cookies()[0]
	service.Close()
	store.Close()
	store, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service = New(store, Options{Registration: true})
	defer service.Close()
	authenticated = httptest.NewRequest("GET", "http://localhost/api/v1/bootstrap", nil)
	authenticated.AddCookie(cookie)
	restored, err := service.Authenticate(authenticated)
	if err != nil || restored.Account.ID != login.Account.ID {
		t.Fatal("restart lost authentication")
	}
	if _, err = service.Login(httptest.NewRecorder(), r, Credentials{Key: "incorrect key"}, false); err == nil {
		t.Fatal("bad password accepted")
	}
}

func TestProxyAwareRateLimits(t *testing.T) {
	for _, trusted := range []bool{false, true} {
		t.Run(fmt.Sprint(trusted), func(t *testing.T) {
			store, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			options := Options{}
			if trusted {
				options.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
			}
			service := New(store, options)
			defer service.Close()
			for i := 0; i < 31; i++ {
				request := httptest.NewRequest("POST", "http://chat/auth/login", nil)
				request.RemoteAddr = "127.0.0.1:1234"
				request.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
				_, err := service.Login(httptest.NewRecorder(), request, Credentials{Key: "ircord-00000-00000-00000-00000-00000-00000"}, false)
				expected := ErrUnauthorized
				if i == 30 && !trusted {
					expected = ErrLimited
				}
				if !errors.Is(err, expected) {
					t.Fatalf("proxy=%v attempt=%d: %v", trusted, i, err)
				}
			}
		})
	}
}
