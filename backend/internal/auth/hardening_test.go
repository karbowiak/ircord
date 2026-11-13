package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"ircord/backend/internal/storage"
)

func hardeningService(t *testing.T, options Options) (*Service, *storage.Store) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	options.Registration = true
	s := New(store, options)
	t.Cleanup(s.Close)
	return s, store
}

// forwarded is a request that reached the backend through the frontend on loopback, for a
// client at the given address.
func forwarded(client string) *http.Request {
	r := httptest.NewRequest("POST", "http://chat/api/v1/auth/login", nil)
	r.RemoteAddr = "127.0.0.1:4000"
	r.Header.Set("X-Forwarded-For", client+", 127.0.0.1")
	return r
}

func loggedIn(t *testing.T, s *Service, r *http.Request, username, password string) Principal {
	t.Helper()
	w := httptest.NewRecorder()
	if _, err := s.Login(w, r, Credentials{Username: username, Password: password}); err != nil {
		t.Fatal(err)
	}
	authenticated := httptest.NewRequest("GET", "http://chat/api/v1/bootstrap", nil)
	authenticated.AddCookie(w.Result().Cookies()[0])
	p, err := s.Authenticate(authenticated)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// One client that keeps failing to sign in must not lock anyone else out of signing in or up.
func TestSignInLimitsApplyPerClient(t *testing.T) {
	s, _ := hardeningService(t, Options{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}})
	if _, err := s.Signup(httptest.NewRecorder(), forwarded("192.0.2.1"), SignupCredentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < signInAttempts; i++ {
		if _, err := s.Login(httptest.NewRecorder(), forwarded("203.0.113.7"), Credentials{Username: "alice", Password: "wrong-password"}); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if _, err := s.Login(httptest.NewRecorder(), forwarded("203.0.113.7"), Credentials{Username: "alice", Password: "wrong-password"}); !errors.Is(err, ErrLimited) {
		t.Fatal("the failing client was not limited", err)
	}
	if _, err := s.Login(httptest.NewRecorder(), forwarded("198.51.100.9"), Credentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal("another client could not sign in", err)
	}
	if _, err := s.Signup(httptest.NewRecorder(), forwarded("192.0.2.44"), SignupCredentials{Username: "bob", Password: "correct-horse"}); err != nil {
		t.Fatal("another client could not sign up", err)
	}
	// Several X-Forwarded-For lines form one chain.
	r := httptest.NewRequest("POST", "http://chat/api/v1/auth/login", nil)
	r.RemoteAddr = "127.0.0.1:4000"
	r.Header.Add("X-Forwarded-For", "203.0.113.7")
	r.Header.Add("X-Forwarded-For", "198.51.100.10")
	if ip := s.clientIP(r); ip != "198.51.100.10" {
		t.Fatal("multi-line forwarded chain read as", ip)
	}
}

// With the Compose default, only the Compose network is trusted: a LAN client behind a reverse
// proxy on the host is counted by its own address, whatever it puts in X-Forwarded-For.
func TestComposeTrustStopsAtTheFirstOutsideAddress(t *testing.T) {
	s, _ := hardeningService(t, Options{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("172.31.250.0/24")}})
	for _, spoof := range []string{"203.0.113.1", "203.0.113.2"} {
		r := httptest.NewRequest("POST", "http://chat/api/v1/auth/login", nil)
		r.RemoteAddr = "172.31.250.3:4000"
		r.Header.Set("X-Forwarded-For", spoof+", 192.168.1.50, 172.31.250.1")
		if ip := s.clientIP(r); ip != "192.168.1.50" {
			t.Fatalf("client chose its own address: %s", ip)
		}
	}
}

// A full attempt table drops the entry that expires soonest instead of refusing everyone.
func TestFullAttemptTableStillAllowsNewClients(t *testing.T) {
	s, _ := hardeningService(t, Options{})
	for i := 0; i < maxTrackedAttempts; i++ {
		s.attempts[fmt.Sprintf("sign-in filler-%d", i)] = attempt{count: 1, until: time.Now().Add(time.Hour + time.Duration(i)*time.Millisecond)}
	}
	if !s.allow("sign-in 198.51.100.9", signInAttempts, time.Minute) {
		t.Fatal("a new client was refused because the table was full")
	}
	if len(s.attempts) != maxTrackedAttempts {
		t.Fatal("the attempt table grew past its limit", len(s.attempts))
	}
	if _, kept := s.attempts["sign-in filler-0"]; kept {
		t.Fatal("the entry that expires soonest was kept")
	}
}

func TestUsernameChecksFollowRegistrationAndLimits(t *testing.T) {
	s, _ := hardeningService(t, Options{})
	for i := 0; i < usernameChecks; i++ {
		if check, err := s.CheckUsername(forwarded("203.0.113.7"), "carol", ""); err != nil || !check.Available {
			t.Fatalf("check %d: %+v %v", i, check, err)
		}
	}
	if _, err := s.CheckUsername(forwarded("203.0.113.7"), "carol", ""); !errors.Is(err, ErrLimited) {
		t.Fatal("username checks were not limited", err)
	}
	// Username checks have their own budget: signing in still works for that client.
	if _, err := s.Login(httptest.NewRecorder(), forwarded("203.0.113.7"), Credentials{Username: "carol", Password: "wrong-password"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("username checks used up sign-in attempts", err)
	}
	closed, _ := hardeningService(t, Options{})
	closed.options.Registration = false
	if _, err := closed.CheckUsername(forwarded("203.0.113.7"), "carol", ""); !errors.Is(err, ErrRegistration) {
		t.Fatal("username check answered while registration is closed", err)
	}
	// With a sign-up key, only requests carrying that key get an answer; wrong keys count as
	// sign-up attempts.
	gated, _ := hardeningService(t, Options{RegistrationKey: "secret-key"})
	for i := 0; i < signInAttempts; i++ {
		if check, err := gated.CheckUsername(forwarded("203.0.113.8"), "carol", "guess"); !errors.Is(err, ErrRegistrationKey) || check.Available {
			t.Fatalf("key-gated check %d answered without the key: %+v %v", i, check, err)
		}
	}
	if _, err := gated.CheckUsername(forwarded("203.0.113.8"), "carol", "guess"); !errors.Is(err, ErrLimited) {
		t.Fatal("wrong sign-up keys were not limited", err)
	}
	if check, err := gated.CheckUsername(forwarded("203.0.113.9"), "carol", "secret-key"); err != nil || !check.Available {
		t.Fatal("the right key got no answer", check, err)
	}
}

// IPv6 clients usually hold a /64; rotating addresses inside it must not reset their limit.
func TestIPv6ClientsShareTheirSlash64Limit(t *testing.T) {
	s, _ := hardeningService(t, Options{})
	request := func(addr string) *http.Request {
		r := httptest.NewRequest("POST", "http://chat/api/v1/auth/login", nil)
		r.RemoteAddr = "[" + addr + "]:4000"
		return r
	}
	for i := 0; i < signInAttempts; i++ {
		if !s.limit(request(fmt.Sprintf("2001:db8:1:2::%x", i+1))) {
			t.Fatalf("attempt %d limited early", i)
		}
	}
	if s.limit(request("2001:db8:1:2:ffff::1")) {
		t.Fatal("a new address in the same /64 started a fresh limit")
	}
	if !s.limit(request("2001:db8:1:3::1")) {
		t.Fatal("another /64 was limited")
	}
}

func TestPasswordChecksAreLimitedPerAccount(t *testing.T) {
	s, _ := hardeningService(t, Options{})
	if _, err := s.Signup(httptest.NewRecorder(), forwarded("192.0.2.1"), SignupCredentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	p := loggedIn(t, s, forwarded("192.0.2.1"), "alice", "correct-horse")
	for i := 0; i < passwordAttempts; i++ {
		if err := s.ChangePassword(t.Context(), p, PasswordChange{Current: "wrong-password", Password: "battery-staple"}); !errors.Is(err, ErrWrongPassword) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if err := s.ChangePassword(t.Context(), p, PasswordChange{Current: "correct-horse", Password: "battery-staple"}); !errors.Is(err, ErrLimited) {
		t.Fatal("password guesses were not limited", err)
	}
	if err := s.DeleteOwn(p, "correct-horse"); !errors.Is(err, ErrLimited) {
		t.Fatal("account deletion skipped the password limit", err)
	}
}

func TestDeletingYourAccountNeedsItsPassword(t *testing.T) {
	s, store := hardeningService(t, Options{})
	if _, err := s.Signup(httptest.NewRecorder(), forwarded("192.0.2.1"), SignupCredentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	p := loggedIn(t, s, forwarded("192.0.2.1"), "alice", "correct-horse")
	if err := s.DeleteOwn(p, "wrong-password"); !errors.Is(err, ErrWrongPassword) {
		t.Fatal("deleted with a wrong password", err)
	}
	if exists, _ := store.AccountExistsByID(p.Account.ID); !exists {
		t.Fatal("a wrong password deleted the account")
	}
	if err := s.DeleteOwn(p, "correct-horse"); err != nil {
		t.Fatal(err)
	}
	if exists, _ := store.AccountExistsByID(p.Account.ID); exists {
		t.Fatal("account not deleted")
	}
}

// When removing an account's files fails after it was marked deleted, the owner is told it
// worked: they are signed out and the cleanup finishes the job.
func TestDeletingYourAccountSucceedsOnceMarkedDeleted(t *testing.T) {
	s, store := hardeningService(t, Options{BeforeDelete: func(string) error { return errors.New("object storage unreachable") }})
	if _, err := s.Signup(httptest.NewRecorder(), forwarded("192.0.2.1"), SignupCredentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	p := loggedIn(t, s, forwarded("192.0.2.1"), "alice", "correct-horse")
	if err := s.DeleteOwn(p, "correct-horse"); err != nil {
		t.Fatal("a deletion the cleanup will finish was reported as failed", err)
	}
	if exists, _ := store.AccountExistsByID(p.Account.ID); exists || s.SessionActive(p) {
		t.Fatal("the account is still usable")
	}
}

// An account whose files can't be removed is retried later; it doesn't stop the cleanup of
// other accounts or fail startup.
func TestAccountCleanupContinuesPastAFailedDeletion(t *testing.T) {
	stuck := ""
	s, store := hardeningService(t, Options{BeforeDelete: func(owner string) error {
		if owner == stuck {
			return errors.New("object storage unreachable")
		}
		return nil
	}})
	ids := map[string]string{}
	for _, name := range []string{"alice", "bob"} {
		v, err := s.Signup(httptest.NewRecorder(), forwarded("192.0.2.1"), SignupCredentials{Username: name, Password: "correct-horse"})
		if err != nil {
			t.Fatal(err)
		}
		ids[name] = v.Account.ID
		if err = store.MarkDeleting(v.Account.ID); err != nil {
			t.Fatal(err)
		}
	}
	stuck = ids["alice"]
	if err := s.PruneExpired(); err != nil {
		t.Fatal("one failed deletion failed the cleanup", err)
	}
	var n int
	store.DB.QueryRow("SELECT count(*) FROM accounts WHERE id=?", ids["bob"]).Scan(&n)
	if n != 0 {
		t.Fatal("the second account was not deleted")
	}
	stuck = ""
	if err := s.PruneExpired(); err != nil {
		t.Fatal(err)
	}
	store.DB.QueryRow("SELECT count(*) FROM accounts WHERE id=?", ids["alice"]).Scan(&n)
	if n != 0 {
		t.Fatal("the failed deletion was not retried")
	}
}

func TestSessionsAreCappedAndPruned(t *testing.T) {
	s, store := hardeningService(t, Options{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}})
	v, err := s.Signup(httptest.NewRecorder(), forwarded("192.0.2.1"), SignupCredentials{Username: "alice", Password: "correct-horse"})
	if err != nil {
		t.Fatal(err)
	}
	owner := v.Account.ID
	// Sign-ins come from many clients so the sign-in limit doesn't interfere.
	for i := 0; i < MaxSessions; i++ {
		if _, err = s.Login(httptest.NewRecorder(), forwarded(fmt.Sprintf("198.51.100.%d", i+1)), Credentials{Username: "alice", Password: "correct-horse"}); err != nil {
			t.Fatal(err)
		}
	}
	live := func() (n int) {
		store.DB.QueryRow("SELECT count(*) FROM sessions WHERE owner=? AND revoked=0", owner).Scan(&n)
		return n
	}
	if n := live(); n != MaxSessions {
		t.Fatal("signed-in devices not capped:", n)
	}
	var first string
	store.DB.QueryRow("SELECT id FROM sessions WHERE owner=? ORDER BY created_ns LIMIT 1", owner).Scan(&first)
	if store.SessionActive(owner, first) {
		t.Fatal("the device that signed in first is still signed in")
	}
	// Revoked sessions are deleted once they are older than the retention period.
	if err = store.PruneSessions(time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var total int
	store.DB.QueryRow("SELECT count(*) FROM sessions WHERE owner=?", owner).Scan(&total)
	if total != MaxSessions || live() != MaxSessions {
		t.Fatal("pruning kept revoked sessions or removed live ones:", total, live())
	}
}
