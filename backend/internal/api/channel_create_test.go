package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestNewPartyCreationIsRetired(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	client := cookieClient()
	register(t, client, server.URL)
	response, err := client.Post(server.URL+"/api/v1/parties", "application/json", strings.NewReader(`{"clientId":"retired","servicesAccounts":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusGone {
		t.Fatal("new party creation was not retired", response.StatusCode)
	}
}
