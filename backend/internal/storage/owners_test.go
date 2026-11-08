package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

func ownersStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestConfiguredOwnersBindToAccountsThatAlreadyExisted(t *testing.T) {
	s := ownersStore(t)
	if _, err := s.CreateAccount("alice-id", "alice"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	unresolved, err := s.ApplyConfiguredOwners([]string{"Alice", "mallory"})
	if err != nil || len(unresolved) != 1 || unresolved[0] != "mallory" {
		t.Fatal("unexpected resolution", unresolved, err)
	}
	if owner, _ := s.IsOwner("alice-id"); !owner {
		t.Fatal("configured existing account isn't an owner")
	}
	// Someone registering a configured name afterwards never becomes an owner, even after restarts.
	if _, err := s.CreateAccount("mallory-id", "mallory"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if unresolved, err = s.ApplyConfiguredOwners([]string{"alice", "mallory"}); err != nil || len(unresolved) != 1 {
			t.Fatal("late registration resolved", unresolved, err)
		}
	}
	if owner, _ := s.IsOwner("mallory-id"); owner {
		t.Fatal("a name registered after configuration became an owner")
	}
	// Owners granted another way survive restarts; dropping a name revokes only that grant.
	if _, err := s.CreateAccount("bob-id", "bob"); err != nil {
		t.Fatal(err)
	}
	if err = s.GrantOwner("bob-id", OwnerFromOwner, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConfiguredOwners(nil); err != nil {
		t.Fatal(err)
	}
	if owner, _ := s.IsOwner("alice-id"); owner {
		t.Fatal("unconfigured environment owner kept ownership")
	}
	if owner, _ := s.IsOwner("bob-id"); !owner {
		t.Fatal("panel-granted owner lost ownership at startup")
	}
}

func TestOwnerRevocationRules(t *testing.T) {
	s := ownersStore(t)
	for _, name := range []string{"alice", "bob", "carol"} {
		if _, err := s.CreateAccount(name+"-id", name); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(time.Millisecond)
	if _, err := s.ApplyConfiguredOwners([]string{"alice"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeOwner("alice-id", false); !errors.Is(err, ErrOwnerFromEnvironment) {
		t.Fatal("environment owner revoked from the panel", err)
	}
	if err := s.GrantOwner("bob-id", OwnerFromOwner, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeOwner("bob-id", false); err != nil {
		t.Fatal(err)
	}
	if err := s.GrantOwner("missing-id", OwnerFromOwner, "alice"); !errors.Is(err, ErrNotFound) {
		t.Fatal("granted a missing account", err)
	}
	// With the environment owner gone, the only remaining owner can't be removed.
	if err := s.GrantOwner("carol-id", OwnerFromOwner, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyConfiguredOwners(nil); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeOwner("carol-id", false); !errors.Is(err, ErrLastOwner) {
		t.Fatal("removed the last owner", err)
	}
	// Deleting an account removes its ownership, and a new account under the name gets nothing.
	if err := s.DeleteAccount("carol-id", false); err != nil {
		t.Fatal(err)
	}
	if owner, _ := s.IsOwner("carol-id"); owner {
		t.Fatal("deleted account kept ownership")
	}
	users, more, err := s.AdminUsers(context.Background(), "", 0, 10)
	if err != nil || more || len(users) != 2 || users[0].Owner || users[0].Username != "alice" {
		t.Fatal("unexpected users", users, more, err)
	}
	if err = s.RecordOwnerEvent(OwnerEvent{Actor: "alice", Action: "owner-grant", Target: "bob", Outcome: "granted"}); err != nil {
		t.Fatal(err)
	}
	if events, err := s.OwnerEvents(context.Background(), 5); err != nil || len(events) != 1 || events[0].Target != "bob" {
		t.Fatal(events, err)
	}
}
