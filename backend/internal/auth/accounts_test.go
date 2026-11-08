package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"ircord/backend/internal/ergo"
	"ircord/backend/internal/storage"
)

// fakeErgo stands in for the associated server's account API.
type fakeErgo struct {
	accounts map[string]string
	down     bool
}

func (f *fakeErgo) Register(_ context.Context, account, passphrase string) error {
	if f.down {
		return errors.New("unreachable")
	}
	if _, ok := f.accounts[strings.ToLower(account)]; ok {
		return ergo.ErrAccountExists
	}
	if strings.HasPrefix(account, "refused") {
		return ergo.ErrAccountRefused
	}
	f.accounts[strings.ToLower(account)] = passphrase
	return nil
}
func (f *fakeErgo) SetPassword(_ context.Context, account, passphrase string) error {
	if f.down {
		return errors.New("unreachable")
	}
	f.accounts[strings.ToLower(account)] = passphrase
	return nil
}
func (f *fakeErgo) AccountExists(_ context.Context, name string) (bool, error) {
	_, ok := f.accounts[strings.ToLower(name)]
	return ok, nil
}

func accountsService(t *testing.T) (*Service, *storage.Store, *fakeErgo, *[]string) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	fake := &fakeErgo{accounts: map[string]string{"taken": "x"}}
	calls := &[]string{}
	s := New(store, Options{Registration: true, Accounts: fake,
		Provision: func(owner, username, password string) error {
			*calls = append(*calls, "provision "+username+" "+password)
			return nil
		},
		PasswordChanged: func(owner, password string) error {
			*calls = append(*calls, "password "+password)
			return nil
		}})
	t.Cleanup(s.Close)
	return s, store, fake, calls
}

func count(t *testing.T, store *storage.Store) int {
	var n int
	if err := store.DB.QueryRow("SELECT count(*) FROM accounts").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSignupCreatesTheIRCAccountAndConnection(t *testing.T) {
	s, store, fake, calls := accountsService(t)
	signup := func(name, password string) error {
		_, err := s.Signup(httptest.NewRecorder(), httptest.NewRequest("POST", "/signup", nil), SignupCredentials{Username: name, Password: password})
		return err
	}
	// Invalid input and names taken on the IRC server create nothing anywhere.
	var invalid InvalidError
	for _, c := range []struct{ name, password string }{{"two words", "correct-horse"}, {"1digit", "correct-horse"}, {"NickServ", "correct-horse"}, {"bob", "short"}, {"bob", "has spaces in it"}} {
		if err := signup(c.name, c.password); !errors.As(err, &invalid) {
			t.Fatalf("%q accepted: %v", c.name, err)
		}
	}
	// Names the IRC server refuses without a reason are reported as such, not as an outage.
	if err := signup("refusedName", "correct-horse"); !errors.As(err, &invalid) || !strings.Contains(invalid.Reason, "username") {
		t.Fatal("refused name not reported", err)
	}
	if err := signup("Taken", "correct-horse"); !errors.Is(err, ErrUsernameTaken) {
		t.Fatal("name taken on the IRC server accepted", err)
	}
	fake.down = true
	if err := signup("alice", "correct-horse"); !errors.Is(err, ErrNetworkUnavailable) {
		t.Fatal("signup succeeded without the IRC server", err)
	}
	if count(t, store) != 0 || len(fake.accounts) != 1 || len(*calls) != 0 {
		t.Fatal("failed signups left state behind")
	}
	fake.down = false
	if err := signup("alice", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	if fake.accounts["alice"] != "correct-horse" || len(*calls) != 1 || (*calls)[0] != "provision alice correct-horse" {
		t.Fatal("IRC account or connection not set up", fake.accounts, *calls)
	}
	if err := signup("ALICE", "correct-horse"); !errors.Is(err, ErrUsernameTaken) {
		t.Fatal("duplicate username in another case accepted", err)
	}
	for name, want := range map[string]bool{"alice": false, "taken": false, "carol": true, "bad name": false} {
		check, err := s.CheckUsername(context.Background(), name)
		if err != nil || check.Available != want || !want && check.Reason == "" {
			t.Fatalf("check %q = %+v, %v", name, check, err)
		}
	}
	if _, hash, err := store.AccountByUsername("alice"); err != nil || !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, "correct-horse") {
		t.Fatal("password not stored as an argon2id hash", err)
	}
}

func TestChangePasswordUpdatesTheIRCAccountAndSignsOutOtherSessions(t *testing.T) {
	s, _, fake, calls := accountsService(t)
	w := httptest.NewRecorder()
	if _, err := s.Signup(w, httptest.NewRequest("POST", "/signup", nil), SignupCredentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	first := httptest.NewRequest("GET", "/", nil)
	first.AddCookie(w.Result().Cookies()[0])
	w = httptest.NewRecorder()
	if _, err := s.Login(w, httptest.NewRequest("POST", "/login", nil), Credentials{Username: "alice", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	second := httptest.NewRequest("GET", "/", nil)
	second.AddCookie(w.Result().Cookies()[0])
	p, err := s.Authenticate(second)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ChangePassword(context.Background(), p, PasswordChange{Current: "wrong-one", Password: "battery-staple"}); !errors.Is(err, ErrWrongPassword) {
		t.Fatal("wrong current password accepted", err)
	}
	if err = s.ChangePassword(context.Background(), p, PasswordChange{Current: "correct-horse", Password: "battery-staple"}); err != nil {
		t.Fatal(err)
	}
	if fake.accounts["alice"] != "battery-staple" || (*calls)[len(*calls)-1] != "password battery-staple" {
		t.Fatal("IRC account or saved login not updated", fake.accounts, *calls)
	}
	if _, err = s.Authenticate(first); err == nil {
		t.Fatal("other session survived a password change")
	}
	if _, err = s.Authenticate(second); err != nil {
		t.Fatal("the session that changed the password was signed out", err)
	}
	if _, err = s.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/login", nil), Credentials{Username: "alice", Password: "correct-horse"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old password still works", err)
	}
	if _, err = s.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/login", nil), Credentials{Username: "alice", Password: "battery-staple"}); err != nil {
		t.Fatal(err)
	}
}
