package irc

import (
	"encoding/json"
	"fmt"
	"github.com/lrstanley/girc"
	"ircord/backend/internal/storage"
	"strings"
	"testing"
)

func TestDiagnosticRedactionAndOwnerIsolation(t *testing.T) {
	manager := New(Options{})
	defer manager.Close()
	w := manager.space("alice")
	w.networks["net"] = Network{ID: "net"}
	w.info["net"] = newConnectionInfo("net")
	c := &connection{owner: w, config: NetworkConfig{ID: "net", SASLPassword: "sasl-secret", ServerPassword: "server-secret"}}
	for _, event := range []girc.Event{{Command: "PASS", Params: []string{"server-secret"}}, {Command: "AUTHENTICATE", Params: []string{"base64-authentication-material"}}, {Command: "OPER", Params: []string{"admin", "operator-secret"}}, {Command: "PRIVMSG", Params: []string{"NickServ", "IDENTIFY nickserv-secret"}}, {Command: "JOIN", Params: []string{"#private", "channel-key"}}, {Command: "NOTICE", Params: []string{"me", "failure echoed sasl-secret"}, Tags: girc.Tags{"label": "sasl-secret"}}} {
		c.capture(event, true)
	}
	c.capture(girc.Event{Command: "PRIVMSG", Params: []string{"#private", "ordinary diagnostic text"}}, false)
	trace, err := manager.ServerLog("alice", "net", "", 100, true)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(trace)
	for _, secret := range []string{"sasl-secret", "server-secret", "base64-authentication-material", "operator-secret", "nickserv-secret", "channel-key"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("diagnostic credential escaped redaction")
		}
	}
	if !strings.Contains(string(raw), "ordinary diagnostic text") {
		t.Fatal("trace hid ordinary IRC traffic")
	}
	if _, err = manager.ConnectionInfo("bob", "net"); err != ErrNotFound {
		t.Fatal("connection diagnostics leaked")
	}
	if _, _, _, _, err = manager.SubscribeTrace("bob", "net", ""); err != ErrNotFound {
		t.Fatal("trace subscription leaked")
	}
	_, live, _, stop, err := manager.SubscribeTrace("alice", "net", "")
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	manager.Close()
	if _, ok := <-live; ok {
		t.Fatal("shutdown did not close trace subscription")
	}
}

func TestLargeListCoalescesLogsWithoutDroppingPagination(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err = store.CreateAccount("alice", "key"); err != nil {
		t.Fatal(err)
	}
	manager := New(Options{Store: store})
	defer manager.Close()
	w := manager.space("alice")
	w.networks["net"] = Network{ID: "net"}
	w.info["net"] = newConnectionInfo("net")
	c := &connection{owner: w, config: NetworkConfig{ID: "net"}}
	c.capture(girc.Event{Command: "321", Params: []string{"me", "Channel", "Users Topic"}}, false)
	before := manager.Snapshot("alice").Cursor
	for i := 0; i < 2000; i++ {
		c.capture(girc.Event{Command: "322", Params: []string{"me", fmt.Sprintf("#channel%d", i), "1", fmt.Sprintf("topic-%d", i)}}, false)
	}
	if manager.Snapshot("alice").Cursor != before {
		t.Fatal("LIST replies flooded the durable event stream")
	}
	c.capture(girc.Event{Command: "323", Params: []string{"me", "End of LIST"}}, false)
	if manager.Snapshot("alice").Cursor != before+1 {
		t.Fatal("LIST completion did not invalidate the server log")
	}
	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 8; i++ {
		page, e := manager.ServerLog("alice", "net", cursor, 100, false)
		if e != nil {
			t.Fatal(e)
		}
		for _, line := range page.Items {
			if seen[line.ID] {
				t.Fatal("server log page duplicated a line")
			}
			seen[line.ID] = true
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if len(seen) != 500 {
		t.Fatalf("retained server window has %d/500 lines", len(seen))
	}
}
