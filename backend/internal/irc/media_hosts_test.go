package irc

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"ircord/backend/internal/storage"
)

func TestNormalMediaHosts(t *testing.T) {
	hosts, err := normalMediaHosts([]string{" Images.Example.com", "", "media.example.org.", "images.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hosts, []string{"images.example.com", "media.example.org"}) {
		t.Fatalf("hosts = %v", hosts)
	}
	for _, good := range []string{"maia.example.co.uk", "foo.github.io", "xn--bcher-kva.de", "cdn.example.dev", "1.example.com"} {
		if _, err := normalMediaHosts([]string{good}); err != nil {
			t.Errorf("%q was refused: %v", good, err)
		}
	}
	if hosts, err := normalMediaHosts(nil); err != nil || hosts == nil || len(hosts) != 0 {
		t.Fatalf("empty = %#v, %v", hosts, err)
	}
	for _, bad := range []string{
		"https://images.example.com", "images.example.com/path", "images.example.com:443",
		"*", "*.example.com", "com", "localhost", "media.localhost", "nas.local", "svc.internal",
		"10.0.0.1", "127.0.0.1", "8.8.8.8", "::1", "[fd00::1]", "user@images.example.com",
		"bad_host.example.com", "-x.example.com", "a..example.com", "images example.com",
		"127.1", "1.1", "1.2.3", "0x7f.1", "host.0x1f", "printer.corp", "x.test", "a.b",
		"files.intranet", "box.private", "co.uk", "github.io", "1.0.0.127.in-addr.arpa",
	} {
		if _, err := normalMediaHosts([]string{"images.example.com", bad}); err == nil {
			t.Errorf("%q was accepted", bad)
		} else if !strings.Contains(err.Error(), strconv.Quote(strings.TrimSpace(bad))) {
			t.Errorf("%q: error does not name the entry: %v", bad, err)
		}
	}
	many := []string{}
	for i := range maxTrustedMediaHosts + 1 {
		many = append(many, "h"+strconv.Itoa(i)+".example.com")
	}
	if _, err := normalMediaHosts(many); err == nil {
		t.Fatal("too many hosts were accepted")
	}
}

// Stored values are re-checked on load, and rows written before the field existed get an
// empty list.
func TestStoredTrustedMediaHostsAreNormalised(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	store.SaveSharedChannel("irc.example.org:6697", "#old", json.RawMessage(`{"notifications":"all","previewMode":"client"}`))
	store.SaveSharedChannel("irc.example.org:6697", "#bad", json.RawMessage(`{"notifications":"all","previewMode":"client","trustedMediaHosts":["images.example.com","127.0.0.1"]}`))
	store.SaveSharedChannel("irc.example.org:6697", "#good", json.RawMessage(`{"notifications":"all","previewMode":"client","trustedMediaHosts":["Images.Example.com"]}`))
	m := New(Options{Store: store})
	defer m.Close()
	for channel, want := range map[string][]string{"#old": {}, "#bad": {}, "#good": {"images.example.com"}, "#unset": {}} {
		got, ok := m.sharedChannel("irc.example.org:6697", channel)
		if !ok || got.TrustedMediaHosts == nil || !slices.Equal(got.TrustedMediaHosts, want) {
			t.Errorf("%s: %#v, want %v", channel, got.TrustedMediaHosts, want)
		}
	}
}
