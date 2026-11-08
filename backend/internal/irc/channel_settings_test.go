package irc

import (
	"encoding/json"
	"strings"
	"testing"

	"ircord/backend/internal/model"
	"ircord/backend/internal/storage"
)

func TestSharedChannelKeysSeparateNetworks(t *testing.T) {
	cfg := func(host string, port int) NetworkConfig { return NetworkConfig{Host: host, Port: port} }
	a := sharedNetworkKey(cfg("IRC.Example.org", 6697), "Example", "")
	if a != sharedNetworkKey(cfg("irc.example.org", 6697), "example", "") {
		t.Fatal("host or NETWORK case split one network")
	}
	for name, other := range map[string]string{
		"another host":                sharedNetworkKey(cfg("irc.other.org", 6697), "Example", ""),
		"another port":                sharedNetworkKey(cfg("irc.example.org", 6667), "Example", ""),
		"another NETWORK on one port": sharedNetworkKey(cfg("irc.example.org", 6697), "Libera", ""),
		"an account-scoped key":       sharedNetworkKey(cfg("irc.example.org", 6697), "Example", "acct"),
	} {
		if other == a {
			t.Fatalf("%s shares channel settings", name)
		}
	}
	if sharedNetworkKey(cfg("b.example", 6697), "X", "alice") == sharedNetworkKey(cfg("b.example", 6697), "X", "bob") {
		t.Fatal("account-scoped keys are shared between accounts")
	}
}

func TestBouncerRoutesAreDetected(t *testing.T) {
	plain := NetworkConfig{Host: "irc.example.org", Port: 6697, Username: "alice", SASLUser: "alice"}
	if bouncerRoute(plain, map[string]bool{"sasl": true, "draft/chathistory": true}, map[string]string{"NETWORK": "Example"}) {
		t.Fatal("a direct connection was treated as a bouncer")
	}
	for name, route := range map[string]struct {
		cfg     NetworkConfig
		caps    map[string]bool
		support map[string]string
	}{
		"soju username":       {NetworkConfig{Username: "alice/libera"}, nil, nil},
		"SASL network suffix": {NetworkConfig{SASLUser: "alice/libera"}, nil, nil},
		"ZNC server password": {NetworkConfig{ServerPassword: "alice/libera:secret"}, nil, nil},
		"auth account":        {NetworkConfig{Auth: &model.AuthInput{Account: "alice/oftc"}}, nil, nil},
		"soju cap":            {plain, map[string]bool{"soju.im/bouncer-networks": true}, nil},
		"ZNC cap":             {plain, map[string]bool{"znc.in/self-message": true}, nil},
		"BOUNCER_NETID":       {plain, nil, map[string]string{"BOUNCER_NETID": "3"}},
	} {
		if !bouncerRoute(route.cfg, route.caps, route.support) {
			t.Fatalf("%s was not detected as a bouncer route", name)
		}
	}
}

func TestLegacyEmbeddingPolicyIsNotExposed(t *testing.T) {
	var conv Conversation
	if err := json.Unmarshal([]byte(`{"id":"one","previewMode":"client","channelSettings":{"notifications":"mentions","previewMode":"client","trustedMediaHosts":["example.com"]}}`), &conv); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(conv)
	if err != nil || strings.Contains(string(raw), "previewMode") || strings.Contains(string(raw), "trustedMediaHosts") || conv.ChannelSettings.Notifications != "mentions" {
		t.Fatal(string(raw), err)
	}
}

func TestSharedChannelSettingsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Store: store})
	if got, ok := m.sharedChannel("irc.example.org:6697", "#room"); !ok || got.Notifications != "default" {
		t.Fatalf("unexpected initial defaults %+v", got)
	}
	raw, _ := json.Marshal(ChannelSettings{Notifications: "mentions", UpdatedBy: "op"})
	if err := store.SaveSharedChannel("irc.example.org:6697", "#room", raw); err != nil {
		t.Fatal(err)
	}
	m.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m = New(Options{Store: store})
	defer m.Close()
	got, _ := m.sharedChannel("irc.example.org:6697", "#room")
	if got.Notifications != "mentions" || got.UpdatedBy != "op" {
		t.Fatalf("settings lost across restart: %+v", got)
	}
	if other, _ := m.sharedChannel("irc.other.org:6697", "#room"); other.Notifications != "default" {
		t.Fatalf("same-named channel on another network inherited settings: %+v", other)
	}
}
