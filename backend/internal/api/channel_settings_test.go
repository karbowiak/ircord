package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"ircord/backend/internal/irc"
	"ircord/backend/internal/notify"
)

// An operator changes shared channel settings; an ordinary member and a new
// joiner see the same values, nobody else may change them, and accounts that
// are not in the channel receive nothing.
func TestErgoSharedChannelSettings(t *testing.T) {
	address := os.Getenv("IRCORD_ERGO_ADDR")
	if address == "" {
		t.Skip("run make test-integration")
	}
	host, portText, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portText)
	app, server, store, _ := authenticatedApp(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	suffix = suffix[len(suffix)-7:]
	channel := "#shared" + suffix

	type person struct {
		client  *http.Client
		account testAccount
		network irc.Network
		nick    string
	}
	connect := func(prefix string) person {
		p := person{client: cookieClient(), nick: prefix + suffix}
		p.account = register(t, p.client, server.URL)
		json.Unmarshal(request(t, p.client, "POST", server.URL+"/api/v1/networks", map[string]any{"name": "Shared", "host": host, "port": port, "tls": false, "nick": p.nick}, 200), &p.network)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := app.Manager.WaitConnected(ctx, p.account.Account.ID, p.network.ID); err != nil {
			t.Fatal(err)
		}
		return p
	}
	join := func(p person) irc.Conversation {
		var conv irc.Conversation
		json.Unmarshal(request(t, p.client, "POST", server.URL+"/api/v1/conversations", map[string]string{"networkId": p.network.ID, "name": channel}, 200), &conv)
		return conv
	}
	current := func(p person, id string) irc.Conversation {
		for _, v := range app.Manager.Snapshot(p.account.Account.ID).Conversations {
			if v.ID == id {
				return v
			}
		}
		return irc.Conversation{}
	}
	settingsOf := func(p person, id string) irc.ChannelSettings {
		if v := current(p, id).ChannelSettings; v != nil {
			return *v
		}
		return irc.ChannelSettings{}
	}
	put := func(p person, id string, body any, status int) []byte {
		return request(t, p.client, "PUT", server.URL+"/api/v1/conversations/"+id+"/channel-settings", body, status)
	}

	op, member, outsider, leaver := connect("cso"), connect("csm"), connect("csx"), connect("csl")
	opConv := join(op)
	waitAPI(t, func() bool {
		v := current(op, opConv.ID)
		return v.Joined && strings.Contains(v.MemberModes[op.account.Account.ID], "@")
	})
	memberConv := join(member)
	waitAPI(t, func() bool { return current(member, memberConv.ID).Joined })
	// The outsider is on the same network, in a different channel; the leaver was in this
	// channel and was kicked before the change. Neither may receive the channel's policy.
	var otherConv irc.Conversation
	json.Unmarshal(request(t, outsider.client, "POST", server.URL+"/api/v1/conversations", map[string]string{"networkId": outsider.network.ID, "name": "#other" + suffix}, 200), &otherConv)
	waitAPI(t, func() bool { return current(outsider, otherConv.ID).Joined && current(outsider, otherConv.ID).ChannelSettings != nil })
	leaverConv := join(leaver)
	waitAPI(t, func() bool { return current(leaver, leaverConv.ID).ChannelSettings != nil })
	// A kick keeps the former member's conversation (not joined), unlike leaving it.
	request(t, op.client, "POST", server.URL+"/api/v1/networks/"+op.network.ID+"/commands", map[string]string{"clientId": "kick" + suffix, "line": "KICK " + channel + " " + leaver.nick}, 202)
	waitAPI(t, func() bool { v := current(leaver, leaverConv.ID); return v.ID != "" && !v.Joined })
	if v := current(leaver, leaverConv.ID); v.ChannelSettings != nil || v.PreviewMode != "" {
		t.Fatalf("a former member still shows channel policy: %q %+v", v.PreviewMode, v.ChannelSettings)
	}
	for _, p := range []struct {
		person
		id string
	}{{op, opConv.ID}, {member, memberConv.ID}} {
		if got := settingsOf(p.person, p.id); got.Notifications != "default" || got.PreviewMode != "client" {
			t.Fatalf("initial shared settings %+v, want default/client", got)
		}
	}

	// Ordinary members cannot change shared settings; foreign IDs are not found.
	put(member, memberConv.ID, map[string]string{"notifications": "all"}, 403)
	put(outsider, opConv.ID, map[string]string{"notifications": "all"}, 404)
	put(op, opConv.ID, map[string]string{"notifications": "loud"}, 422)
	// The old per-account channel preview switch no longer writes a channel value, and a
	// refused request applies none of its other fields either.
	request(t, member.client, "PATCH", server.URL+"/api/v1/conversations/"+memberConv.ID, map[string]any{"favorite": true, "previewMode": "server"}, 422)
	if current(member, memberConv.ID).Favorite {
		t.Fatal("a refused PATCH still applied favorite")
	}

	var changed irc.Conversation
	json.Unmarshal(put(op, opConv.ID, map[string]string{"notifications": "mentions", "previewMode": "server"}, 200), &changed)
	if changed.ChannelSettings == nil || changed.ChannelSettings.Notifications != "mentions" || changed.PreviewMode != "server" || changed.ChannelSettings.UpdatedBy != op.nick {
		t.Fatalf("operator update returned %+v", changed.ChannelSettings)
	}
	waitAPI(t, func() bool {
		got := settingsOf(member, memberConv.ID)
		return got.Notifications == "mentions" && got.PreviewMode == "server" && current(member, memberConv.ID).PreviewMode == "server"
	})
	// A partial update keeps the other value.
	put(op, opConv.ID, map[string]string{"notifications": "all"}, 200)
	waitAPI(t, func() bool {
		got := settingsOf(member, memberConv.ID)
		return got.Notifications == "all" && got.PreviewMode == "server"
	})
	// Setting the current value again changes nothing: no new timestamp, so nothing to broadcast.
	stamp := settingsOf(member, memberConv.ID).UpdatedAt
	var same irc.Conversation
	json.Unmarshal(put(op, opConv.ID, map[string]string{"notifications": "all"}, 200), &same)
	time.Sleep(300 * time.Millisecond)
	if got := settingsOf(member, memberConv.ID).UpdatedAt; stamp == nil || got == nil || !got.Equal(*stamp) || !same.ChannelSettings.UpdatedAt.Equal(*stamp) {
		t.Fatal("a no-op change was written or broadcast")
	}

	// The member's change arrived through its own durable event stream.
	events, _, err := store.Replay(member.account.Account.ID, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	delivered := false
	for _, e := range events {
		if e.Type == "conversation.updated" && strings.Contains(string(e.Data), `"notifications":"all"`) {
			delivered = true
		}
	}
	if !delivered {
		t.Fatal("shared settings were not part of the member's replayable events")
	}
	// Neither the account in another channel nor the former member received this channel's policy.
	for _, p := range []person{outsider, leaver} {
		events, _, err = store.Replay(p.account.Account.ID, 0, 1000)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range events {
			if strings.Contains(string(e.Data), `"updatedBy":"`+op.nick+`"`) {
				t.Fatalf("%s received channel settings it is not entitled to: %s", p.nick, e.Data)
			}
		}
	}
	if got := settingsOf(outsider, otherConv.ID); got.Notifications != "default" || got.PreviewMode != "client" || got.UpdatedBy != "" {
		t.Fatalf("another channel took this channel's settings: %+v", got)
	}

	// Saved notification settings keep personal levels for direct messages only.
	var dm irc.Conversation
	json.Unmarshal(request(t, member.client, "POST", server.URL+"/api/v1/conversations", map[string]string{"networkId": member.network.ID, "memberId": op.nick}, 200), &dm)
	var saved struct {
		Levels map[string]string `json:"levels"`
	}
	rules := notify.Defaults()
	rules.Levels = map[string]string{memberConv.ID: "none", dm.ID: "none"}
	json.Unmarshal(request(t, member.client, "PUT", server.URL+"/api/v1/notifications/settings", rules, 200), &saved)
	if len(saved.Levels) != 1 || saved.Levels[dm.ID] != "none" {
		t.Fatalf("saved levels %v, want only the direct message", saved.Levels)
	}

	// Extra trusted media hosts: operators only, validated, shared with every member and
	// limited to this channel.
	hosts := map[string]any{"trustedMediaHosts": []string{"Images.Example.com", "images.example.com"}}
	put(member, memberConv.ID, hosts, 403)
	if refused := string(put(op, opConv.ID, map[string]any{"trustedMediaHosts": []string{"https://images.example.com/a"}}, 400)); !strings.Contains(refused, "without scheme, port or path") {
		t.Fatalf("invalid host refusal is unclear: %s", refused)
	}
	put(op, opConv.ID, map[string]any{"trustedMediaHosts": []string{"*"}}, 400)
	put(op, opConv.ID, map[string]any{"trustedMediaHosts": []string{"10.1.2.3"}}, 400)
	json.Unmarshal(put(op, opConv.ID, hosts, 200), &changed)
	if got := changed.ChannelSettings; got == nil || len(got.TrustedMediaHosts) != 1 || got.TrustedMediaHosts[0] != "images.example.com" || got.Notifications != "all" {
		t.Fatalf("trusted media hosts update returned %+v", got)
	}
	waitAPI(t, func() bool {
		got := settingsOf(member, memberConv.ID).TrustedMediaHosts
		return len(got) == 1 && got[0] == "images.example.com"
	})
	if got := settingsOf(outsider, otherConv.ID).TrustedMediaHosts; got == nil || len(got) != 0 {
		t.Fatalf("another channel took this channel's trusted media hosts: %#v", got)
	}

	// A new joiner gets the current values.
	outsiderConv := join(outsider)
	waitAPI(t, func() bool {
		got := settingsOf(outsider, outsiderConv.ID)
		return current(outsider, outsiderConv.ID).Joined && got.Notifications == "all" && got.PreviewMode == "server" && len(got.TrustedMediaHosts) == 1
	})

	// Losing operator status removes the right to change the settings.
	request(t, op.client, "POST", server.URL+"/api/v1/networks/"+op.network.ID+"/commands", map[string]string{"clientId": "deop" + suffix, "line": "MODE " + channel + " -o " + op.nick}, 202)
	waitAPI(t, func() bool { return !strings.Contains(current(op, opConv.ID).MemberModes[op.account.Account.ID], "@") })
	put(op, opConv.ID, map[string]string{"notifications": "none"}, 403)
	put(op, opConv.ID, map[string]any{"trustedMediaHosts": []string{}}, 403)
	if got := settingsOf(member, memberConv.ID); got.Notifications != "all" || len(got.TrustedMediaHosts) != 1 {
		t.Fatalf("a denied change reached members: %+v", got)
	}
}
