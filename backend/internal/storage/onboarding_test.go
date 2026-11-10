package storage

import (
	"encoding/json"
	"testing"
)

func TestAccountOnboardingMigrationKeepsExistingAccountsIneligible(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	account(t, s, "prior")
	// Model the last published schema, with an existing account and no setup column.
	if _, err := s.DB.Exec("ALTER TABLE accounts DROP COLUMN onboarding_network_id; DELETE FROM goose_db_version WHERE version_id=35"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, _, err := s.AccountByUsername("test-key-hash-prior")
	if err != nil || a.OnboardingNetworkID != "" {
		t.Fatalf("existing account enrolled: %+v, %v", a, err)
	}
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["onboardingNetworkId"]; exists {
		t.Fatal("ineligible account exposes a pending network")
	}
}

func TestAccountOnboardingInsertIsAtomicAndDurable(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A failure after the account INSERT must roll back both eligibility and the account.
	if _, err := s.DB.Exec(`CREATE TRIGGER fail_profile BEFORE INSERT ON entities BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUserWithOnboarding("fresh", "fresh", "fixture-hash", "home"); err == nil {
		t.Fatal("injected failure succeeded")
	}
	if exists, err := s.AccountExistsByID("fresh"); err != nil || exists {
		t.Fatal("partial account insertion survived", err)
	}
	if _, err := s.DB.Exec("DROP TRIGGER fail_profile"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUserWithOnboarding("fresh", "fresh", "fixture-hash", "home"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	a, _, err := s.AccountByUsername("fresh")
	if err != nil || a.OnboardingNetworkID != "home" {
		t.Fatalf("restart lost pending setup: %+v, %v", a, err)
	}
	for range 2 {
		if err := s.CompleteOnboarding(a.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	s = reopened
	a, _, err = s.AccountByUsername("fresh")
	if err != nil || a.OnboardingNetworkID != "" {
		t.Fatalf("restart rearmed completed setup: %+v, %v", a, err)
	}
}
