package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/irc"
)

// The theme library is one more account document: private to its owner,
// saved with compare-and-swap, capped at 1 MiB, and announced to the owner's
// other devices as themes.updated.
func TestThemesDocument(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	alice, bob := cookieClient(), cookieClient()
	register(t, alice, server.URL)
	register(t, bob, server.URL)
	url := server.URL + "/api/v1/app-data/themes"

	var document AppDocument
	json.Unmarshal(request(t, alice, "GET", url, nil, 200), &document)
	if string(document.Data) != "null" {
		t.Fatal("new account has themes", string(document.Data))
	}
	socket := accountSocket(t, alice, server.URL, 0)
	old := document.Revision
	document.Data = json.RawMessage(`{"format":1,"customCss":true,"themes":[{"id":"t1","name":"Night","css":".chat-main{color:red}"}]}`)
	json.Unmarshal(request(t, alice, "PUT", url, document, 200), &document)
	if document.Revision == old || !strings.Contains(string(document.Data), "Night") {
		t.Fatal("theme library not saved", string(document.Data))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, raw, err := socket.Read(ctx)
		if err != nil {
			t.Fatal("no themes.updated event", err)
		}
		if strings.Contains(string(raw), `"themes.updated"`) {
			if !strings.Contains(string(raw), document.Revision) || strings.Contains(string(raw), "Night") {
				t.Fatal("themes event must carry only the revision", string(raw))
			}
			break
		}
	}

	// A device that saves from an older copy is refused rather than overwriting.
	request(t, alice, "PUT", url, AppDocument{Revision: old, Data: json.RawMessage(`{"themes":[]}`)}, 409)

	// Other accounts never see the library.
	var other AppDocument
	json.Unmarshal(request(t, bob, "GET", url, nil, 200), &other)
	if string(other.Data) != "null" {
		t.Fatal("theme library leaked to another account")
	}

	// Themes are capped at 1 MiB; the same document fits the other kinds' 4 MiB.
	big := json.RawMessage(`{"themes":[],"css":"` + strings.Repeat("a", 1<<20) + `"}`)
	request(t, alice, "PUT", url, AppDocument{Revision: document.Revision, Data: big}, 400)
	var library AppDocument
	json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/app-data/library", nil, 200), &library)
	request(t, alice, "PUT", server.URL+"/api/v1/app-data/library", AppDocument{Revision: library.Revision, Data: big}, 200)
	var kept AppDocument
	json.Unmarshal(request(t, alice, "GET", url, nil, 200), &kept)
	if kept.Revision != document.Revision {
		t.Fatal("rejected theme library replaced the saved one")
	}
}

// The operator can switch custom CSS off for the instance; clients learn it
// from the bootstrap capabilities.
func TestCustomCSSCapability(t *testing.T) {
	app, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	register(t, client, server.URL)
	themes := func() *bool {
		t.Helper()
		var snapshot irc.Snapshot
		if err := json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/bootstrap", nil, 200), &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Capabilities.Themes == nil {
			return nil
		}
		return &snapshot.Capabilities.Themes.CustomCSS
	}
	if allowed := themes(); allowed == nil || !*allowed {
		t.Fatal("custom CSS must be on by default")
	}
	app.options.DisableCustomCSS = true
	if allowed := themes(); allowed == nil || *allowed {
		t.Fatal("operator switch did not turn custom CSS off")
	}
}
