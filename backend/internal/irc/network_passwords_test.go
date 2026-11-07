package irc

import (
	"encoding/json"
	"strings"
	"testing"

	"ircord/backend/internal/model"
)

func TestOperatorPasswordBoundToTLSEndpoint(t *testing.T) {
	passwords, err := ParseNetworkPasswords([]byte(`{"IRC.EXAMPLE.ORG:6697":"operator-secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{NetworkPasswords: passwords})
	defer m.Close()
	cfg := NetworkConfig{ID: "net", Name: "Example", Host: "irc.example.org", Port: 6697, TLS: true, Nick: "tester", Auth: &model.AuthInput{Method: "server-password"}}
	configured := m.configuredPassword(cfg)
	if configured.Auth.Password == nil || *configured.Auth.Password != "operator-secret" || cfg.Auth.Password != nil {
		t.Fatal("default missing or caller mutated")
	}
	for _, change := range []func(*NetworkConfig){func(c *NetworkConfig) { c.TLS = false }, func(c *NetworkConfig) { c.Host = "elsewhere.example" }, func(c *NetworkConfig) { c.Port = 6698 }, func(c *NetworkConfig) { c.Auth = &model.AuthInput{Method: "nickserv"} }} {
		next := clone(cfg)
		change(&next)
		if m.configuredPassword(next).Auth.Password != nil {
			t.Fatal("default escaped its exact TLS endpoint")
		}
	}
	empty := ""
	cleared := clone(cfg)
	cleared.Auth.Password = &empty
	if *m.configuredPassword(cleared).Auth.Password != "" {
		t.Fatal("explicit password clearing was overridden")
	}
	w := m.space("owner")
	w.configs[cfg.ID] = configured
	w.networks[cfg.ID] = Network{ID: cfg.ID}
	next := clone(configured)
	next.Host = "elsewhere.example"
	result, err := m.UpdateNetwork("owner", cfg.ID, next)
	if err != nil {
		t.Fatal(err)
	}
	if result.Auth.PasswordSet || w.configs[cfg.ID].Auth.Password != nil {
		t.Fatal("changing the endpoint retained a secret")
	}
	var public Network
	applyConfig(&public, configured)
	raw, _ := json.Marshal(public)
	if strings.Contains(string(raw), "operator-secret") || !public.Auth.PasswordSet {
		t.Fatal("public auth summary exposed or lost the secret")
	}
	for _, raw := range []string{`{"bad":"secret"}`, `{"irc.example:6697":"bad\nsecret"}`, `{"irc.example:70000":"secret"}`} {
		if _, err := ParseNetworkPasswords([]byte(raw)); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("invalid secret config accepted or echoed")
		}
	}
}
