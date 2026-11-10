package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type lostSignupResponse struct{ *fakeErgo }

func (f lostSignupResponse) Register(ctx context.Context, username, password string) error {
	if err := f.fakeErgo.Register(ctx, username, password); err != nil {
		return err
	}
	return errors.New("registration response lost")
}

func TestOnboardingUnconfirmedUpstreamCreationStaysIneligible(t *testing.T) {
	s, store, fake, _ := accountsService(t)
	s.options.OnboardingNetworkID = "home"
	s.options.Accounts = lostSignupResponse{fake}
	if _, err := s.Signup(httptest.NewRecorder(), httptest.NewRequest("POST", "/signup", nil), SignupCredentials{Username: "ambiguous", Password: "correct-horse"}); !errors.Is(err, ErrNetworkUnavailable) {
		t.Fatal(err)
	}
	restarted := New(store, s.options)
	defer restarted.Close()
	if err := restarted.RecoverCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := restarted.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/login", nil), Credentials{Username: "ambiguous", Password: "correct-horse"})
	if err != nil || result.Account.OnboardingNetworkID != "" {
		t.Fatal("unconfirmed creation enrolled an imported account", err)
	}
}

func TestOnboardingRequiresConfirmedNewDefaultServerSignup(t *testing.T) {
	for _, tc := range []struct {
		name       string
		associated bool
		provision  bool
		home       string
		imported   bool
		want       string
	}{
		{name: "fresh", associated: true, provision: true, home: "home", want: "home"},
		{name: "imported", associated: true, provision: true, home: "home", imported: true},
		{name: "no-default", associated: true, provision: true},
		{name: "unassociated", provision: true, home: "home"},
		{name: "no-provision", associated: true, home: "home"},
		{name: "demo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, fake, _ := accountsService(t)
			s.options.OnboardingNetworkID = tc.home
			if !tc.associated {
				s.options.Accounts = nil
			}
			if !tc.provision {
				s.options.Provision = nil
			}
			if tc.imported {
				fake.accounts["onboard"] = "correct-horse"
			}
			result, err := s.Signup(httptest.NewRecorder(), httptest.NewRequest("POST", "/signup", nil), SignupCredentials{Username: "onboard", Password: "correct-horse"})
			if err != nil || result.Account.OnboardingNetworkID != tc.want {
				t.Fatalf("signup pending network = %q, want %q: %v", result.Account.OnboardingNetworkID, tc.want, err)
			}
			// Provisioning a connection by itself cannot enroll or rearm an account.
			if s.options.Provision != nil {
				if err := s.options.Provision(result.Account.ID, "onboard", "correct-horse"); err != nil {
					t.Fatal(err)
				}
			}
			login, err := s.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/login", nil), Credentials{Username: "onboard", Password: "correct-horse"})
			if err != nil || login.Account.OnboardingNetworkID != tc.want {
				t.Fatalf("login pending network = %q, want %q: %v", login.Account.OnboardingNetworkID, tc.want, err)
			}
		})
	}
}

func TestOnboardingPersistsAcrossSessionsAndCompletion(t *testing.T) {
	s, store, _, _ := accountsService(t)
	// A prior IRCord account with zero networks must stay ineligible when a home is configured.
	prior, err := s.Signup(httptest.NewRecorder(), httptest.NewRequest("POST", "/signup", nil), SignupCredentials{Username: "prior", Password: "correct-horse"})
	if err != nil {
		t.Fatal(err)
	}
	s.options.OnboardingNetworkID = "home"
	if login, err := s.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/login", nil), Credentials{Username: "prior", Password: "correct-horse"}); err != nil || login.Account.OnboardingNetworkID != "" || login.Account.ID != prior.Account.ID {
		t.Fatal("prior account became eligible", err)
	}
	w := httptest.NewRecorder()
	if result, err := s.Signup(w, httptest.NewRequest("POST", "/signup", nil), SignupCredentials{Username: "fresh", Password: "correct-horse"}); err != nil || result.Account.OnboardingNetworkID != "home" {
		t.Fatal("fresh account missing setup", err)
	}
	r := httptest.NewRequest("GET", "/session", nil)
	r.AddCookie(w.Result().Cookies()[0])
	restarted := New(store, s.options)
	defer restarted.Close()
	p, err := restarted.Authenticate(r)
	if err != nil || p.Account.OnboardingNetworkID != "home" {
		t.Fatal("service restart lost setup", err)
	}
	second := httptest.NewRecorder()
	if login, err := restarted.Login(second, httptest.NewRequest("POST", "/login", nil), Credentials{Username: "fresh", Password: "correct-horse"}); err != nil || login.Account.OnboardingNetworkID != "home" {
		t.Fatal("new device lost setup", err)
	}
	for range 2 {
		if err := restarted.CompleteOnboarding(p); err != nil {
			t.Fatal("completion is not idempotent", err)
		}
	}
	otherDevice := httptest.NewRequest("GET", "/session", nil)
	otherDevice.AddCookie(second.Result().Cookies()[0])
	for _, request := range []*http.Request{r, otherDevice} {
		if p, err := restarted.Authenticate(request); err != nil || p.Account.OnboardingNetworkID != "" {
			t.Fatal("completion not visible to session", err)
		}
	}
	if err := restarted.Revoke(p.Account.ID, p.Session.ID); err != nil {
		t.Fatal(err)
	}
	if err := restarted.CompleteOnboarding(p); !errors.Is(err, ErrSession) {
		t.Fatal("revoked session completed setup", err)
	}
}

func TestOnboardingRecoveryNeverRearmsExistingAccount(t *testing.T) {
	s, store, _, _ := accountsService(t)
	s.options.OnboardingNetworkID = "home"
	s.options.Provision = func(string, string, string) error { return errors.New("injected failure") }
	if _, err := s.Signup(httptest.NewRecorder(), httptest.NewRequest("POST", "/signup", nil), SignupCredentials{Username: "recover", Password: "correct-horse"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	a, _, err := store.AccountByUsername("recover")
	if err != nil || a.OnboardingNetworkID != "home" {
		t.Fatal("partial signup lost pending setup", err)
	}
	// A retained, positively classified operation must never restore a cleared marker.
	if err := store.CompleteOnboarding(a.ID); err != nil {
		t.Fatal(err)
	}
	s.options.Provision = func(string, string, string) error { return nil }
	restarted := New(store, s.options)
	defer restarted.Close()
	if err := restarted.RecoverCredentials(context.Background()); err != nil {
		t.Fatal(err)
	}
	login, err := restarted.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/login", nil), Credentials{Username: "recover", Password: "correct-horse"})
	if err != nil || login.Account.OnboardingNetworkID != "" {
		t.Fatal("recovery rearmed completed account", err)
	}
}
