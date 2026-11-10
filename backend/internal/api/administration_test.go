package api

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"ircord/backend/internal/storage"
)

func TestAdministrationIsOwnerOnlyAndRevocationIsImmediate(t *testing.T) {
	_, server, store, _ := authenticatedApp(t)
	ownerClient, otherClient := cookieClient(), cookieClient()
	owner := register(t, ownerClient, server.URL)
	other := register(t, otherClient, server.URL)
	base := server.URL + administrationPrefix

	// Nobody owns the server yet: everyone is refused, but can ask whether they have access.
	if body := request(t, otherClient, "GET", base+"/access", nil, 200); !strings.Contains(string(body), `"owner":false`) {
		t.Fatal("access check", string(body))
	}
	for _, path := range []string{"/status", "/database", "/users", "/identities?query=t", "/audit"} {
		request(t, otherClient, "GET", base+path, nil, 403)
	}
	request(t, otherClient, "POST", base+"/owners", map[string]string{"account": other.Account.ID}, 403)
	request(t, cookieClient(), "GET", base+"/access", nil, 401)

	if err := store.GrantOwner(owner.Account.ID, storage.OwnerFromOperator, "operator"); err != nil {
		t.Fatal(err)
	}
	if body := request(t, ownerClient, "GET", base+"/access", nil, 200); !strings.Contains(string(body), `"owner":true`) {
		t.Fatal("owner access", string(body))
	}
	var status struct {
		Status   string
		Overview struct{ Accounts int }
	}
	if err := json.Unmarshal(request(t, ownerClient, "GET", base+"/status", nil, 200), &status); err != nil || status.Overview.Accounts != 2 {
		t.Fatal("status", status, err)
	}
	var page AdminUserPage
	var database DatabaseStatus
	if err := json.Unmarshal(request(t, ownerClient, "GET", base+"/database", nil, 200), &database); err != nil || !database.Available || database.Pool.MaxOpen != 1 {
		t.Fatal("database diagnostics", database, err)
	}
	if err := json.Unmarshal(request(t, ownerClient, "GET", base+"/users?query="+url.QueryEscape(other.Username), nil, 200), &page); err != nil || len(page.Items) != 1 || page.Items[0].ID != other.Account.ID {
		t.Fatal("user search", page, err)
	}
	// Users carry no credentials.
	if body := string(request(t, ownerClient, "GET", base+"/users", nil, 200)); strings.Contains(body, "argon2") || strings.Contains(body, "password") {
		t.Fatal("credential material in the user list")
	}
	var matches struct{ Items []IdentityMatch }
	if err := json.Unmarshal(request(t, ownerClient, "GET", base+"/identities?query="+url.QueryEscape(other.Username[:6]), nil, 200), &matches); err != nil || len(matches.Items) < 2 || !matches.Items[0].Eligible {
		t.Fatal("identities", matches, err)
	}

	// Granting by a username or display text isn't possible: only an account ID resolves.
	request(t, ownerClient, "POST", base+"/owners", map[string]string{"account": other.Username}, 404)
	request(t, ownerClient, "POST", base+"/owners", map[string]string{"account": other.Account.ID, "reason": "helps run the server"}, 200)
	request(t, otherClient, "GET", base+"/status", nil, 200)

	// Revoking takes effect on the very next request of an existing session.
	request(t, ownerClient, "DELETE", base+"/owners/"+other.Account.ID+"?reason=stepping+down", nil, 200)
	request(t, otherClient, "GET", base+"/status", nil, 403)
	// The last owner can't be removed, not even by themselves.
	request(t, ownerClient, "DELETE", base+"/owners/"+owner.Account.ID, nil, 409)

	var audit struct{ Items []storage.OwnerEvent }
	if err := json.Unmarshal(request(t, ownerClient, "GET", base+"/audit", nil, 200), &audit); err != nil || len(audit.Items) != 3 {
		t.Fatal("audit", audit, err)
	}
	if e := audit.Items[1]; e.Actor != owner.Username || e.Action != "owner-revoke" || e.Target != other.Username || e.Reason != "stepping down" || e.Outcome != "revoked" {
		t.Fatal("audit entry", e)
	}
}
