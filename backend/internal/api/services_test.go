package api

import (
	"ircord/backend/internal/irc"
	"testing"
)

func TestChannelServicesAuthenticationAndOwnerScope(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	signed := cookieClient()
	register(t, signed, server.URL)
	for _, v := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/registration", nil}, {"POST", "/registration", nil}, {"DELETE", "/registration", nil},
		{"GET", "/access", nil}, {"PUT", "/access/alice", map[string]string{"mode": "v"}}, {"DELETE", "/access/alice", nil},
		{"POST", "/transfer", map[string]string{"account": "alice"}},
	} {
		path := server.URL + "/api/v1/conversations/another-owner-channel" + v.path
		request(t, cookieClient(), v.method, path, v.body, 401)
		request(t, signed, v.method, path, v.body, 404)
	}
}
func TestServiceProblemStatuses(t *testing.T) {
	for _, status := range []int{403, 404, 409, 422, 501, 502, 504} {
		result, err := serviceResult(false, &irc.ServiceError{Status: status, Detail: "A safe explanation."})
		if result != nil || err == nil {
			t.Fatal("service error lost")
		}
		// Huma's status-bearing errors keep the specific protocol result.
		e, ok := err.(interface{ GetStatus() int })
		if !ok || e.GetStatus() != status {
			t.Fatal("wrong service HTTP status", status, err)
		}
	}
}
