package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ircord/backend/internal/storage"
)

func signIn(t *testing.T, s *Service, username, password string, signup bool) *http.Request {
	t.Helper()
	w := httptest.NewRecorder()
	var err error
	if signup {
		_, err = s.Signup(w, httptest.NewRequest("POST", "/", nil), SignupCredentials{Username: username, Password: password})
	} else {
		_, err = s.Login(w, httptest.NewRequest("POST", "/", nil), Credentials{Username: username, Password: password})
	}
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(w.Result().Cookies()[0])
	return r
}

func login(s *Service, username, password string) error {
	_, err := s.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil), Credentials{Username: username, Password: password})
	return err
}

func auditLog(t *testing.T, store *storage.Store) []string {
	t.Helper()
	events, err := store.AdminEvents(100)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].At.IsZero() {
			t.Fatal("audit event without a time")
		}
		out = append(out, events[i].Action+" "+events[i].Target+" "+events[i].Outcome)
	}
	return out
}

func TestResetPasswordConvergesAndSignsOutEverySession(t *testing.T) {
	s, store, fake, calls := accountsService(t)
	first := signIn(t, s, "alice", "correct-horse", true)
	second := signIn(t, s, "alice", "correct-horse", false)
	other := signIn(t, s, "bob", "bobs-password", true)
	p, err := s.Authenticate(first)
	if err != nil {
		t.Fatal(err)
	}
	live, unwatch, err := s.Watch(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer unwatch()
	*calls = nil
	result, err := s.ResetPassword(context.Background(), " ALICE ", PasswordReset{Password: "operator-chosen"})
	if err != nil {
		t.Fatal(err)
	}
	if result != (PasswordResetResult{Username: "alice", Status: ResetCompleted, SessionsRevoked: 2, Associated: true}) {
		t.Fatalf("result = %+v", result)
	}
	if fake.accounts["alice"] != "operator-chosen" || len(*calls) != 1 || (*calls)[0] != "password operator-chosen" {
		t.Fatal("IRC account or saved login not updated", fake.accounts, *calls)
	}
	for _, r := range []*http.Request{first, second} {
		if _, err := s.Authenticate(r); err == nil {
			t.Fatal("a prior session survived the reset")
		}
	}
	if live.Err() == nil {
		t.Fatal("the live connection of a prior session stayed open")
	}
	if !errors.Is(login(s, "alice", "correct-horse"), ErrUnauthorized) {
		t.Fatal("old password still works")
	}
	if err := login(s, "alice", "operator-chosen"); err != nil {
		t.Fatal(err)
	}
	// Other accounts are untouched.
	if _, err := s.Authenticate(other); err != nil || fake.accounts["bob"] != "bobs-password" || login(s, "bob", "bobs-password") != nil {
		t.Fatal("an unrelated account changed", err)
	}
	if ops, err := store.CredentialOperations(); err != nil || len(ops) != 0 {
		t.Fatal("completed intent retained", err)
	}
	if got := auditLog(t, store); len(got) != 1 || got[0] != "password-reset alice completed" {
		t.Fatal("audit log", got)
	}
}

func TestResetPasswordRejectsBadInputWithoutChanges(t *testing.T) {
	s, store, fake, _ := accountsService(t)
	session := signIn(t, s, "alice", "correct-horse", true)
	var invalid InvalidError
	for _, password := range []string{"short", "has spaces in it", strings.Repeat("x", 129)} {
		if _, err := s.ResetPassword(context.Background(), "alice", PasswordReset{Password: password}); !errors.As(err, &invalid) {
			t.Fatalf("%q accepted: %v", password, err)
		}
	}
	if _, err := s.ResetPassword(context.Background(), "nobody", PasswordReset{Password: "operator-chosen"}); !errors.Is(err, ErrNoAccount) {
		t.Fatal("unknown account", err)
	}
	if _, err := s.Authenticate(session); err != nil || fake.accounts["alice"] != "correct-horse" || login(s, "alice", "correct-horse") != nil {
		t.Fatal("a refused reset changed the account", err)
	}
	got := auditLog(t, store)
	if len(got) != 4 || got[3] != "password-reset nobody failed" {
		t.Fatal("audit log", got)
	}
	for _, line := range got {
		if strings.Contains(line, "short") || strings.Contains(line, "operator-chosen") {
			t.Fatal("credential material in the audit log")
		}
	}
}

func TestResetPasswordIsPendingUntilTheIRCServerAgrees(t *testing.T) {
	s, store, fake, calls := accountsService(t)
	session := signIn(t, s, "alice", "correct-horse", true)
	fake.down = true
	*calls = nil
	result, err := s.ResetPassword(context.Background(), "alice", PasswordReset{Password: "operator-chosen"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != ResetPending || result.SessionsRevoked != 1 || result.Detail == "" {
		t.Fatalf("result = %+v", result)
	}
	if _, err := s.Authenticate(session); err == nil {
		t.Fatal("a prior session survived a pending reset")
	}
	// Neither password signs in while IRCord and the IRC server disagree.
	for _, password := range []string{"correct-horse", "operator-chosen"} {
		if err := login(s, "alice", password); err == nil {
			t.Fatal("sign-in allowed while the reset is pending")
		}
	}
	if len(*calls) != 0 || fake.accounts["alice"] != "correct-horse" {
		t.Fatal("pending reset changed the saved login")
	}
	var raw []byte
	if err := store.DB.QueryRow("SELECT payload FROM credential_operations").Scan(&raw); err != nil || strings.Contains(string(raw), "operator-chosen") {
		t.Fatal("intent missing or unencrypted", err)
	}
	fake.down = false
	restarted := New(store, s.options)
	defer restarted.Close()
	if err := restarted.RecoverCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.accounts["alice"] != "operator-chosen" || len(*calls) != 1 || (*calls)[0] != "password operator-chosen" {
		t.Fatal("recovery did not finish the reset", fake.accounts, *calls)
	}
	if !errors.Is(login(restarted, "alice", "correct-horse"), ErrUnauthorized) || login(restarted, "alice", "operator-chosen") != nil {
		t.Fatal("recovered reset left the wrong password")
	}
	if got := auditLog(t, store); len(got) != 2 || got[0] != "password-reset alice pending" || got[1] != "password-reset alice completed" {
		t.Fatal("audit log", got)
	}
	// A retried reset with the same password is safe and completes.
	if result, err := restarted.ResetPassword(context.Background(), "alice", PasswordReset{Password: "operator-chosen"}); err != nil || result.Status != ResetCompleted || result.SessionsRevoked != 1 {
		t.Fatal("retry", result, err)
	}
}

func TestResetPasswordSupersedesAPendingChange(t *testing.T) {
	s, _, fake, _ := accountsService(t)
	p, err := s.Authenticate(signIn(t, s, "alice", "correct-horse", true))
	if err != nil {
		t.Fatal(err)
	}
	fake.down = true
	if err := s.ChangePassword(context.Background(), p, PasswordChange{Current: "correct-horse", Password: "users-choice"}); !errors.Is(err, ErrNetworkUnavailable) {
		t.Fatal(err)
	}
	fake.down = false
	if result, err := s.ResetPassword(context.Background(), "alice", PasswordReset{Password: "operator-chosen"}); err != nil || result.Status != ResetCompleted {
		t.Fatal(result, err)
	}
	if fake.accounts["alice"] != "operator-chosen" || login(s, "alice", "operator-chosen") != nil || login(s, "alice", "users-choice") == nil {
		t.Fatal("the reset didn't win over the pending change")
	}
}

func TestConcurrentResetsAgreeOnOnePassword(t *testing.T) {
	s, store, fake, _ := accountsService(t)
	signIn(t, s, "alice", "correct-horse", true)
	passwords := []string{"first-choice", "second-choice", "third-choice", "fourth-choice"}
	var wg sync.WaitGroup
	for _, password := range passwords {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.ResetPassword(context.Background(), "alice", PasswordReset{Password: password}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	final := fake.accounts["alice"]
	for _, password := range passwords {
		if (login(s, "alice", password) == nil) != (password == final) {
			t.Fatal("IRCord and the IRC server disagree after concurrent resets", final)
		}
	}
	if ops, err := store.CredentialOperations(); err != nil || len(ops) != 0 {
		t.Fatal("intent retained", err)
	}
}

func TestResetPasswordWithoutAnAssociatedServer(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	s := New(store, Options{Registration: true})
	t.Cleanup(s.Close)
	signIn(t, s, "alice", "correct-horse", true)
	result, err := s.ResetPassword(context.Background(), "alice", PasswordReset{Password: "operator-chosen"})
	if err != nil || result.Status != ResetCompleted || result.Associated {
		t.Fatal(result, err)
	}
	if login(s, "alice", "operator-chosen") != nil {
		t.Fatal("new password rejected")
	}
}
