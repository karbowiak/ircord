package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"ircord/backend/internal/irc"
)

func TestReplayBarrierPrecedesLiveAndHandlesEmptyReplay(t *testing.T) {
	app, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	account := register(t, client, server.URL)
	owner := account.Account.ID
	for _, empty := range []bool{false, true} {
		cursor := app.Manager.Cursor(owner)
		if !empty {
			if _, err := app.Manager.SetProfile(owner, irc.Profile{Bio: "while away"}); err != nil {
				t.Fatal(err)
			}
		}
		head := app.Manager.Cursor(owner)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/events?sync=1", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{server.URL}}})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		read := func() map[string]any {
			t.Helper()
			_, raw, err := socket.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var event map[string]any
			if err = json.Unmarshal(raw, &event); err != nil {
				t.Fatal(err)
			}
			return event
		}
		if ready := read(); ready["type"] != "session.ready" || ready["sync"] != true {
			t.Fatal("missing synchronization capability")
		}
		resume, _ := json.Marshal(sessionResume{V: 1, Type: "session.resume", After: cursor, Sync: true})
		if err := socket.Write(ctx, websocket.MessageText, resume); err != nil {
			t.Fatal(err)
		}
		var barrier map[string]any
		last := cursor
		for {
			event := read()
			if event["type"] == "session.synced" {
				barrier = event
				break
			}
			seq := uint64(event["seq"].(float64))
			if seq != last+1 || seq > head {
				t.Fatal("replay is not contiguous")
			}
			last = seq
		}
		if last != head || uint64(barrier["cursor"].(float64)) != head {
			t.Fatal("barrier did not identify the completed replay")
		}
		if _, err := app.Manager.SetProfile(owner, irc.Profile{Bio: "now live"}); err != nil {
			t.Fatal(err)
		}
		if event := read(); event["type"] != "profile.updated" || uint64(event["seq"].(float64)) != head+1 {
			t.Fatal("live event did not follow the replay barrier")
		}
		socket.CloseNow()
		cancel()
	}
}
