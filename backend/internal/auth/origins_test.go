package auth

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"ircord/backend/internal/storage"
)

func TestVerifiedOriginSignupIsolationAndCoalescing(t *testing.T) {
	var store *storage.Store
	var provisionOrigin string
	s, db := hardeningService(t, Options{TrackOrigins: true, TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
		Provision: func(owner, _, _ string) error { var err error; provisionOrigin, err = store.Origin(owner); return err }})
	store = db
	w := httptest.NewRecorder()
	alice, err := s.Signup(w, forwarded("198.51.100.1"), SignupCredentials{Username: "alice", Password: "correct-horse"})
	if err != nil {
		t.Fatal(err)
	}
	if provisionOrigin != "198.51.100.1" {
		t.Fatalf("home provision had origin %q", provisionOrigin)
	}
	cookie := w.Result().Cookies()[0]
	bob, err := s.Signup(httptest.NewRecorder(), forwarded("198.51.100.2"), SignupCredentials{Username: "bob", Password: "correct-horse"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Login(httptest.NewRecorder(), forwarded("198.51.100.99"), Credentials{Username: "alice", Password: "wrong-password"}); err == nil {
		t.Fatal("invalid login accepted")
	}
	if got, _ := s.Origin(alice.Account.ID); got != "198.51.100.1" {
		t.Fatal("failed login changed origin")
	}
	for i := 0; i < 30; i++ {
		address := "203.0.113.1"
		if i%2 == 1 {
			address = "203.0.113.2"
		}
		r := forwarded(address)
		r.AddCookie(cookie)
		if _, err := s.Authenticate(r); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := s.Origin(alice.Account.ID); got != "203.0.113.2" {
		t.Fatalf("latest origin = %q", got)
	}
	if got, _ := store.Origin(alice.Account.ID); got != "198.51.100.1" {
		t.Fatalf("coalescing persisted intermediate origins: %q", got)
	}
	if got, _ := s.Origin(bob.Account.ID); got != "198.51.100.2" {
		t.Fatalf("other owner origin = %q", got)
	}
	s.origins.flush()
	restarted := New(store, Options{TrackOrigins: true})
	if got, _ := restarted.Origin(alice.Account.ID); got != "203.0.113.2" {
		t.Fatalf("restored origin = %q", got)
	}
	restarted.Close()
	if err := s.Delete(alice.Account.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Origin(alice.Account.ID); got != "" {
		t.Fatal("deleted account retained origin")
	}
}

func TestVerifiedOriginUsesTrustedResolver(t *testing.T) {
	s, _ := hardeningService(t, Options{TrackOrigins: true, TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("127.0.0.0/8")}})
	for _, tc := range []struct {
		name, remote, forwarded, want string
		verified                      bool
	}{
		{"untrusted spoof", "192.0.2.5:22", "198.51.100.9", "192.0.2.5", true},
		{"trusted chain", "10.1.0.2:22", "198.51.100.5, 10.2.0.3", "198.51.100.5", true},
		{"mapped ipv4", "10.1.0.2:22", "::ffff:198.51.100.5", "198.51.100.5", true},
		{"malformed", "10.1.0.2:22", "198.51.100.5, bad", "10.1.0.2", false},
		{"all trusted", "10.1.0.2:22", "10.2.0.3", "10.1.0.2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "http://chat/", nil)
			r.RemoteAddr = tc.remote
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			got, valid := s.resolvedClientIP(r)
			if got != tc.want || valid != tc.verified {
				t.Fatalf("got %q, %v", got, valid)
			}
		})
	}
}

func TestVerifiedOriginAvailableBeforeSignInReconnect(t *testing.T) {
	var service *Service
	var signInOrigin string
	service, _ = hardeningService(t, Options{TrackOrigins: true, TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}, SignedIn: func(owner string) error {
		var err error
		signInOrigin, err = service.Origin(owner)
		return err
	}})
	if _, err := service.Signup(httptest.NewRecorder(), forwarded("198.51.100.1"), SignupCredentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Login(httptest.NewRecorder(), forwarded("198.51.100.7"), Credentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	if signInOrigin != "198.51.100.7" {
		t.Fatalf("sign-in reconnect observed %q", signInOrigin)
	}
}
