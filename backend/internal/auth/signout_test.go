package auth

import (
	"net/http/httptest"
	"slices"
	"testing"

	"ircord/backend/internal/storage"
)

// Signing out of the last device tells IRC to go offline (#237); signing out of one of two
// devices doesn't, and every sign-in tells IRC to come back.
func TestOnlyTheLastSignOutTakesTheAccountOffIRC(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	calls := []string{}
	service := New(store, Options{Registration: true,
		SignedOut: func(owner string) error { calls = append(calls, "out:"+owner); return nil },
		SignedIn:  func(owner string) error { calls = append(calls, "in:"+owner); return nil },
	})
	laptop := signedIn(t, service, "laptop")
	owner := laptop.Account.ID
	login := func() Principal {
		t.Helper()
		w := httptest.NewRecorder()
		if _, err := service.Login(w, httptest.NewRequest("POST", "http://localhost/auth/login", nil), Credentials{Username: "alice", Password: "correct-horse"}); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("GET", "http://localhost/api/v1/bootstrap", nil)
		req.AddCookie(w.Result().Cookies()[0])
		p, err := service.Authenticate(req)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	phone := login()
	if want := []string{"in:" + owner, "in:" + owner}; !slices.Equal(calls, want) {
		t.Fatalf("sign-ins reported %v, want %v", calls, want)
	}
	calls = nil
	if err = service.Logout(httptest.NewRecorder(), laptop); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("signing out with another device still signed in reported %v", calls)
	}
	if err = service.Logout(httptest.NewRecorder(), phone); err != nil {
		t.Fatal(err)
	}
	if want := []string{"out:" + owner}; !slices.Equal(calls, want) {
		t.Fatalf("last sign-out reported %v, want %v", calls, want)
	}

	// Signing the last device out from the device list counts too.
	calls = nil
	tablet := login()
	devices, err := service.Devices(tablet)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range devices.Devices {
		if d.Current {
			if _, err = service.RevokeDevice(tablet, d.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if want := []string{"in:" + owner, "out:" + owner}; !slices.Equal(calls, want) {
		t.Fatalf("signing out from the device list reported %v, want %v", calls, want)
	}
}
