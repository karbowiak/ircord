package auth

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"ircord/backend/internal/storage"
)

func TestRegistrationGateRejectsBeforeCreatingStateAndPreservesLogin(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s := New(store, Options{Registration: true, RegistrationKey: "fixture-registration-key"})
	defer s.Close()
	policy, _ := json.Marshal(s.RegistrationPolicy())
	if !strings.Contains(string(policy), `"registrationKeyRequired":true`) || strings.Contains(string(policy), "fixture-registration-key") {
		t.Fatal("unsafe registration policy")
	}
	for _, value := range []string{"", "wrong", "Fixture-registration-key"} {
		w := httptest.NewRecorder()
		_, err := s.Login(w, httptest.NewRequest("POST", "/register", nil), Credentials{RegistrationKey: value}, true)
		if !errors.Is(err, ErrRegistrationKey) || len(w.Result().Cookies()) != 0 {
			t.Fatal("invalid registration key accepted", err)
		}
	}
	for _, table := range []string{"accounts", "sessions"} {
		var count int
		if err := store.DB.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("failed registrations created state", table, count, err)
		}
	}
	result, err := s.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/register", nil), Credentials{RegistrationKey: "fixture-registration-key"}, true)
	if err != nil || result.Key == "" {
		t.Fatal("correct key rejected", err)
	}
	login, err := s.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/login", nil), Credentials{Key: result.Key}, false)
	if err != nil || login.Account.ID != result.Account.ID {
		t.Fatal("existing login now requires the registration key", err)
	}
}

func TestRegistrationRequiredWithoutSecretFailsClosedAndRetainsRateLimit(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s := New(store, Options{Registration: true, RequireRegistrationKey: true})
	defer s.Close()
	for i := 0; i < 31; i++ {
		_, err := s.Login(httptest.NewRecorder(), httptest.NewRequest("POST", "/register", nil), Credentials{}, true)
		if i < 30 && !errors.Is(err, ErrRegistrationKey) || i == 30 && !errors.Is(err, ErrLimited) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
}
