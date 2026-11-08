package irc

import (
	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
	"testing"
)

func TestHomePasswordBoundToEndpointAndAccount(t *testing.T) {
	for _, field := range []string{"host", "port", "tls", "account", "method", "unchanged"} {
		t.Run(field, func(t *testing.T) {
			m := New(Options{})
			defer m.Close()
			w := m.space("owner")
			old := "old-password"
			home := NetworkConfig{ID: "home", Name: "Home", Host: "irc.example.org", Port: 6697, TLS: true, Nick: "alice"}
			cfg := clone(home)
			cfg.Auth = &model.AuthInput{Method: "sasl-plain", Account: "alice", Password: &old}
			switch field {
			case "host":
				cfg.Host = "elsewhere.example"
			case "port":
				cfg.Port = 6698
			case "tls":
				cfg.TLS = false
			case "account":
				cfg.Auth.Account = "bob"
			case "method":
				cfg.Auth.Method = "nickserv"
			}
			w.configs[home.ID] = cfg
			w.networks[home.ID] = Network{ID: home.ID}
			if err := m.UpdateHomePassword("owner", home, "alice", "new-password"); err != nil {
				t.Fatal(err)
			}
			got := *w.configs[home.ID].Auth.Password
			if field == "unchanged" && got != "new-password" || field != "unchanged" && got != "old-password" {
				t.Fatal("password escaped destination check")
			}
		})
	}
}
func TestHomeNetworkPersistsAtConnectionLimit(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.CreateUser("owner", "alice", "hash"); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store, MaxConnections: 1})
	defer m.Close()
	m.connectionCount = 1
	password := "test-password"
	cfg := NetworkConfig{ID: "home", Name: "Home", Host: "irc.example.org", Port: 6697, TLS: true, Nick: "alice", Auth: &model.AuthInput{Method: "sasl-plain", Account: "alice", Password: &password}}
	if err := m.ProvisionHome("owner", cfg); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Networks("owner")
	if err != nil || len(saved) != 1 || !saved[0].Enabled || saved[0].Config.SASLPassword != password {
		t.Fatal("network lost when pool full", err)
	}
}
