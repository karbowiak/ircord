package main

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestConfigurePreservesDefaultsAndIsolatesTestServer(t *testing.T) {
	raw := []byte(`network:
  name: Original
server:
  name: original.example
  listeners:
    ":7000": {}
  unrelated-setting: keep
accounts:
  registration:
    enabled: true
history:
  enabled: false
opers:
  example: {}
limits:
  nicklen: 32
`)
	out, err := configure(raw)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	at := func(path ...string) any {
		var value any = cfg
		for _, key := range path {
			mapping, ok := value.(map[string]any)
			if !ok {
				t.Fatalf("%v is not a mapping", path)
			}
			value = mapping[key]
		}
		return value
	}
	for _, tc := range []struct {
		path []string
		want any
	}{
		{[]string{"network", "name"}, "IRCordTest"},
		{[]string{"server", "name"}, "ergo.test"},
		{[]string{"server", "unrelated-setting"}, "keep"},
		{[]string{"accounts", "registration", "enabled"}, true},
		{[]string{"limits", "nicklen"}, 32},
		{[]string{"history", "persistent", "enabled"}, true},
		{[]string{"accounts", "multiclient", "always-on"}, "opt-in"},
		{[]string{"server", "relaymsg", "enabled"}, false},
		{[]string{"fakelag", "enabled"}, false},
		{[]string{"server", "listeners", ":6697", "tls", "cert"}, "fullchain.pem"},
	} {
		if got := at(tc.path...); got != tc.want {
			t.Errorf("%v = %v; want %v", tc.path, got, tc.want)
		}
	}
	if len(at("opers").(map[string]any)) != 0 {
		t.Fatal("test configuration retained operators")
	}
	listeners := at("server", "listeners").(map[string]any)
	if len(listeners) != 2 || listeners[":6667"] == nil {
		t.Fatal("test configuration retained other listeners")
	}
}

func TestConfigureRejectsInvalidInput(t *testing.T) {
	for _, raw := range []string{"", "null", "- item", "server: ["} {
		if _, err := configure([]byte(raw)); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
