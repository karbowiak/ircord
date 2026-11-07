package admin

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTokenSourcesAndVerification(t *testing.T) {
	token := strings.Repeat("ab", 32)
	file := filepath.Join(t.TempDir(), "admin-token")
	if err := os.WriteFile(file, []byte(token+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, source := range []struct{ value, file string }{{token, ""}, {"", file}} {
		got, err := LoadToken(source.value, source.file)
		if err != nil || got != token {
			t.Fatal("credential source failed", err)
		}
	}
	if value, err := LoadToken("", ""); err != nil || value != "" {
		t.Fatal("unconfigured administration must remain disabled")
	}
	if _, err := LoadToken(token, file); err == nil || strings.Contains(err.Error(), token) {
		t.Fatal("ambiguous credential configuration was accepted or leaked")
	}
	v, err := NewVerifier(token)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		header string
		want   bool
	}{{"", false}, {"Bearer " + token, true}, {"bearer " + token, true}, {"Basic " + token, false}, {"Bearer " + strings.Repeat("cd", 32), false}, {"Bearer bot-other-principal", false}, {"Bearer " + token + " extra", false}} {
		r := httptest.NewRequest("GET", Prefix+"/status", nil)
		r.Header.Set("Authorization", tc.header)
		if v.Authenticate(r) != tc.want {
			t.Fatal("unexpected bearer authentication result")
		}
		var disabled *Verifier
		if disabled.Authenticate(r) {
			t.Fatal("disabled verifier authenticated")
		}
	}
	r := httptest.NewRequest("GET", Prefix+"/status?token="+token, nil)
	r.Header.Add("Authorization", "Bearer "+token)
	r.Header.Add("Authorization", "Bearer "+token)
	if v.Authenticate(r) {
		t.Fatal("duplicate authorization headers accepted")
	}
	rotated, _ := NewVerifier(strings.Repeat("cd", 32))
	r.Header.Set("Authorization", "Bearer "+token)
	if rotated.Authenticate(r) {
		t.Fatal("previous credential accepted after rotation")
	}
}

func TestInvalidCredentialFilesNeverLeakValues(t *testing.T) {
	for _, value := range []string{"", "short-secret", strings.Repeat("z", 64), strings.Repeat("a", 4097)} {
		file := filepath.Join(t.TempDir(), "credential")
		if err := os.WriteFile(file, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadToken("", file)
		if err == nil || value != "" && strings.Contains(err.Error(), value) {
			t.Fatal("invalid credential accepted or disclosed")
		}
	}
	for _, path := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing")} {
		if _, err := LoadToken("", path); err == nil {
			t.Fatal("invalid file accepted")
		}
	}
	if IsPath("/api/v1/administrator") || !IsPath(Prefix+"/future-command") {
		t.Fatal("incorrect admin route boundary")
	}
}
