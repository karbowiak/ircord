package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestSignupRecoversAfterLocalCommitFailure(t *testing.T) {
	s, store, fake, _ := accountsService(t)
	s.options.OnboardingNetworkID = "home"
	if _, err := store.DB.Exec(`CREATE TRIGGER fail_account BEFORE INSERT ON accounts BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	in := SignupCredentials{Username: "recover", Password: "recovery-password", DisplayName: "Recovered"}
	if _, err := s.Signup(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil), in); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if fake.accounts["recover"] != in.Password || count(t, store) != 0 {
		t.Fatal("wrong partial state")
	}
	var raw []byte
	if err := store.DB.QueryRow("SELECT payload FROM credential_operations").Scan(&raw); err != nil || strings.Contains(string(raw), in.Password) {
		t.Fatal("intent missing or unencrypted", err)
	}
	if _, err := store.DB.Exec("DROP TRIGGER fail_account"); err != nil {
		t.Fatal(err)
	}
	if op, err := s.loadOperation(in.Username); err != nil || op == nil || op.OnboardingNetworkID != "home" {
		t.Fatal("confirmed signup eligibility was not saved before local insertion", err)
	}
	// Recreate the service to model process recovery rather than relying on memory.
	restarted := New(store, s.options)
	defer restarted.Close()
	if err := restarted.RecoverCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	if result, err := restarted.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil), Credentials{Username: in.Username, Password: in.Password}); err != nil || result.Account.OnboardingNetworkID != "home" {
		t.Fatal("recovery lost confirmed signup eligibility", err)
	}
	if ops, err := store.CredentialOperations(); err != nil || len(ops) != 0 {
		t.Fatal("completed intent retained", err)
	}
}

func TestPasswordRecoveryRetriesSavedLoginAndRevokesSessions(t *testing.T) {
	s, store, fake, _ := accountsService(t)
	w := httptest.NewRecorder()
	if _, err := s.Signup(w, httptest.NewRequest("POST", "/", nil), SignupCredentials{Username: "recover", Password: "original-password"}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(w.Result().Cookies()[0])
	p, err := s.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	s.options.PasswordChanged = func(string, string) error { return errors.New("injected save failure") }
	if err := s.ChangePassword(context.Background(), p, PasswordChange{Current: "original-password", Password: "replacement-password"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if fake.accounts["recover"] != "replacement-password" {
		t.Fatal("upstream not changed")
	}
	saved := ""
	s.options.PasswordChanged = func(_, password string) error { saved = password; return nil }
	restarted := New(store, s.options)
	defer restarted.Close()
	if err := restarted.RecoverCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	if saved != "replacement-password" {
		t.Fatal("saved login not repaired")
	}
	if _, err := restarted.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil), Credentials{Username: "recover", Password: "original-password"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old password accepted", err)
	}
	if _, err := restarted.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil), Credentials{Username: "recover", Password: "replacement-password"}); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentPasswordChangesUseCurrentPasswordAndSession(t *testing.T) {
	s, _, fake, _ := accountsService(t)
	w := httptest.NewRecorder()
	if _, err := s.Signup(w, httptest.NewRequest("POST", "/", nil), SignupCredentials{Username: "race", Password: "original-password"}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(w.Result().Cookies()[0])
	p, err := s.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, password := range []string{"replacement-first", "replacement-second"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.ChangePassword(context.Background(), p, PasswordChange{Current: "original-password", Password: password})
		}()
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrWrongPassword) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatal("concurrent changes both succeeded")
	}
	if _, err := s.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil), Credentials{Username: "race", Password: fake.accounts["race"]}); err != nil {
		t.Fatal("upstream and local passwords diverged", err)
	}
	if err := s.Revoke(p.Account.ID, p.Session.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangePassword(context.Background(), p, PasswordChange{Current: fake.accounts["race"], Password: "another-password"}); !errors.Is(err, ErrSession) {
		t.Fatal("revoked session changed password", err)
	}
}

func TestDisplayNameValidationMatchesProfileByteLimit(t *testing.T) {
	if DisplayNameProblem(strings.Repeat("é", 41)) == "" || DisplayNameProblem(strings.Repeat("é", 40)) != "" {
		t.Fatal("UTF-8 limit mismatch")
	}
}
