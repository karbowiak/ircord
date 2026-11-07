package api

import (
	"context"
	"encoding/json"
	"io"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/preview"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type publicResolver struct{}

func (publicResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
}

type previewTransport func(*http.Request) (*http.Response, error)

func (f previewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestErgoPreviewPolicyAndDurableMetadata(t *testing.T) {
	address := os.Getenv("IRCORD_ERGO_ADDR")
	if address == "" {
		t.Skip("run make test-integration")
	}
	host, portText, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portText)
	var calls atomic.Int32
	service := preview.New(preview.Options{Resolver: publicResolver{}, Client: &http.Client{Transport: previewTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(`<head><meta property="og:title" content="Preview from the backend"></head>`)), Request: r}, nil
	})}})
	app, server, store, _ := authenticatedApp(t, func(options *irc.Options) { options.Previews = service })
	alice, other := cookieClient(), cookieClient()
	account := register(t, alice, server.URL)
	register(t, other, server.URL)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	suffix = suffix[len(suffix)-7:]
	var network irc.Network
	json.Unmarshal(request(t, alice, "POST", server.URL+"/api/v1/networks", map[string]any{"name": "Preview", "host": host, "port": port, "tls": false, "nick": "prev" + suffix}, 200), &network)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := app.Manager.WaitConnected(ctx, account.Account.ID, network.ID); err != nil {
		t.Fatal(err)
	}
	var conv irc.Conversation
	json.Unmarshal(request(t, alice, "POST", server.URL+"/api/v1/conversations", map[string]string{"networkId": network.ID, "name": "preview" + suffix}, 200), &conv)
	waitAPI(t, func() bool {
		for _, v := range app.Manager.Snapshot(account.Account.ID).Conversations {
			if v.ID == conv.ID {
				return v.Joined
			}
		}
		return false
	})
	raw := request(t, alice, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/preview?url=https%3A%2F%2Fpublic.example%2Farticle", nil, 200)
	if !strings.Contains(string(raw), `"mode":"client"`) || calls.Load() != 0 {
		t.Fatal("client mode fetched metadata on the backend")
	}
	request(t, other, "PATCH", server.URL+"/api/v1/conversations/"+conv.ID, map[string]string{"previewMode": "server"}, 404)
	request(t, alice, "PATCH", server.URL+"/api/v1/conversations/"+conv.ID, map[string]string{"previewMode": "server"}, 200)
	request(t, alice, "POST", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", map[string]string{"clientId": "preview-send", "text": "https://public.example/article"}, 202)
	waitAPI(t, func() bool {
		command, err := store.Command(account.Account.ID, "preview-send")
		if err != nil || command.MessageID == "" {
			return false
		}
		msg, err := store.Message(account.Account.ID, conv.ID, command.MessageID)
		return err == nil && msg.Preview != nil && msg.Preview.Title == "Preview from the backend" && msg.PreviewSource == "server"
	})
	if calls.Load() != 1 {
		t.Fatal("unexpected duplicate preview requests")
	}
	request(t, other, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/preview?url=https%3A%2F%2Fpublic.example%2Farticle", nil, 404)
	var history irc.MessagePage
	json.Unmarshal(request(t, alice, "GET", server.URL+"/api/v1/conversations/"+conv.ID+"/messages", nil, 200), &history)
	found := false
	for _, msg := range history.Items {
		if msg.Preview != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("preview omitted from durable history")
	}
}
