package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestPingNeedsSessionAndReportsServerTime(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	request(t, cookieClient(), "GET", server.URL+"/api/v1/ping", nil, 401)
	client := cookieClient()
	register(t, client, server.URL)
	before := time.Now().Add(-time.Second)
	var ping Ping
	if err := json.Unmarshal(request(t, client, "GET", server.URL+"/api/v1/ping", nil, 200), &ping); err != nil {
		t.Fatal(err)
	}
	if ping.ServerTime.Before(before) || ping.ServerTime.After(time.Now().Add(time.Second)) {
		t.Fatalf("server time %s", ping.ServerTime)
	}
	response, err := client.Get(server.URL + "/api/v1/ping")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("status %d, cache %q", response.StatusCode, response.Header.Get("Cache-Control"))
	}
}
