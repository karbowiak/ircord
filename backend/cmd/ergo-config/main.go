// Generate the local integration configuration from the pinned image's defaults.
package main

import (
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"
)

func main() {
	raw, err := io.ReadAll(os.Stdin)
	must(err)
	b, err := configure(raw)
	must(err)
	fmt.Print(string(b))
}

func configure(raw []byte) ([]byte, error) {
	var cfg map[string]any
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("configuration must be an Ergo YAML mapping")
	}
	set := func(path []string, value any) {
		m := cfg
		for _, key := range path[:len(path)-1] {
			next, ok := m[key].(map[string]any)
			if !ok {
				next = map[string]any{}
				m[key] = next
			}
			m = next
		}
		m[path[len(path)-1]] = value
	}
	set([]string{"network", "name"}, "IRCordTest")
	set([]string{"server", "name"}, "ergo.test")
	set([]string{"server", "listeners"}, map[string]any{":6667": map[string]any{}, ":6697": map[string]any{"tls": map[string]any{"cert": "fullchain.pem", "key": "privkey.pem"}}})
	set([]string{"server", "ip-limits", "count"}, false)
	set([]string{"server", "ip-limits", "throttle"}, false)
	set([]string{"fakelag", "enabled"}, false)
	set([]string{"metadata", "enabled"}, true)
	set([]string{"metadata", "client-throttle", "enabled"}, false)
	set([]string{"accounts", "registration", "throttling", "enabled"}, false)
	set([]string{"accounts", "multiclient", "enabled"}, true)
	set([]string{"accounts", "multiclient", "always-on"}, "opt-in")
	set([]string{"history", "enabled"}, true)
	set([]string{"history", "persistent", "enabled"}, true)
	set([]string{"history", "persistent", "unregistered-channels"}, true)
	set([]string{"history", "retention", "allow-individual-delete"}, true)
	set([]string{"history", "retention", "enable-account-indexing"}, true)
	set([]string{"history", "tagmsg-storage", "default"}, false)
	set([]string{"history", "tagmsg-storage", "whitelist"}, []string{"+draft/react", "+draft/unreact", "+react", "+unreact"})
	set([]string{"datastore", "sqlite", "enabled"}, true)
	set([]string{"languages", "path"}, "/ircd-bin/languages")
	// No operator credentials, HTTP API, web push, or relay gateway are needed.
	cfg["opers"] = map[string]any{}
	set([]string{"server", "relaymsg", "enabled"}, false)
	return yaml.Marshal(cfg)
}
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
