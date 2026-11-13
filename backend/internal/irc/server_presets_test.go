package irc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeServerList(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "servers.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestServerListStrictAndPrivate(t *testing.T) {
	t.Setenv("TEST_WEBIRC_PASSWORD", "fixture-gateway-secret")
	data := "servers:\n  - name: Example\n    host: IRC.Example.\n    authMethod: server-password\n    autoJoin: ['#private']\nwebirc:\n  - host: irc.example\n    passwordEnv: TEST_WEBIRC_PASSWORD\n"
	list, err := LoadServerList(writeServerList(t, data), map[string]string{"irc.example:6697": "fixture-server-password"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !list.Configured || len(list.Presets) != 1 || !list.Presets[0].PasswordProvided || list.Presets[0].Host != "irc.example" || !list.Presets[0].TLS {
		t.Fatalf("bad public preset: %+v", list.Presets)
	}
	if _, ok := list.gateway("IRC.EXAMPLE.", 6697, true); !ok {
		t.Fatal("canonical endpoint did not match")
	}
	for _, x := range []struct {
		host string
		port int
		tls  bool
	}{{"other.example", 6697, true}, {"irc.example", 6667, true}, {"irc.example", 6697, false}} {
		if _, ok := list.gateway(x.host, x.port, x.tls); ok {
			t.Fatal("gateway matched another endpoint")
		}
	}
	raw, _ := json.Marshal(list.Presets)
	for _, private := range []string{"fixture-gateway-secret", "fixture-server-password", "TEST_WEBIRC_PASSWORD", "webirc"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("public presets exposed operator data")
		}
	}
	bad := []string{
		"servers:\n  - name: X\n    host: example.com\n    unsafe: secret-value\n",
		"servers:\n  - name: X\n    host: example.com\n  - name: Y\n    host: EXAMPLE.COM.\n",
		"servers:\n  - name: X\n    host: example.com\n    port: 0\n",
		"servers:\n  - name: X\n    host: 127.0.0.999\n",
		"webirc:\n  - host: example.com\n    password: inline-secret\n",
		"webirc:\n  - host: example.com\n    passwordEnv: MISSING_FIXTURE_ENV\n",
		"webirc:\n  - host: example.com\n    tls: false\n    passwordEnv: TEST_WEBIRC_PASSWORD\n",
		"webirc:\n  - host: example.com\n    passwordEnv: TEST_WEBIRC_PASSWORD\n  - host: EXAMPLE.COM.\n    passwordEnv: TEST_WEBIRC_PASSWORD\n",
		"servers: []\n---\nservers: []\n",
	}
	for _, text := range bad {
		_, err := LoadServerList(writeServerList(t, text), nil, false)
		if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "MISSING") {
			t.Fatalf("unsafe config error: %v", err)
		}
	}
	empty, err := LoadServerList(writeServerList(t, "servers: []\n"), nil, false)
	if err != nil || !empty.Configured || len(empty.Presets) != 0 {
		t.Fatal("explicit empty list", err)
	}
	absent, err := LoadServerList("", nil, false)
	if err != nil || absent.Configured || len(absent.Presets) != 0 {
		t.Fatal("absent list", err)
	}
}

func TestWebIRCPlaintextRequiresResolvedPrivateAddress(t *testing.T) {
	_, err := (safeDialer{ctx: context.Background(), allowPrivate: true, requirePrivate: true}).Dial("tcp", "8.8.8.8:6667")
	if err != errWebIRCPrivateOnly {
		t.Fatalf("public plaintext destination: %v", err)
	}
}

func TestServerListBoundsAndSecrets(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "credential")
	if err := os.WriteFile(secretPath, []byte("fixture-file-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	text := fmt.Sprintf("webirc:\n  - host: example.org\n    passwordFile: %q\n", secretPath)
	list, err := LoadServerList(writeServerList(t, text), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	gateway, ok := list.gateway("example.org", 6697, true)
	if !ok || gateway.Password != "fixture-file-secret" {
		t.Fatal("file secret unavailable")
	}
	if _, err = LoadServerList(writeServerList(t, strings.Repeat("x", 65537)), nil, false); err == nil {
		t.Fatal("oversized YAML accepted")
	}
}
