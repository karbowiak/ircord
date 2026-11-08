package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestHashFieldScoresAreSharedValidatedAndRateLimited(t *testing.T) {
	_, server, _, _ := authenticatedApp(t)
	url := server.URL + "/api/v1/hash-field/scores"
	anonymous := cookieClient()
	request(t, anonymous, "GET", url, nil, 401)
	request(t, anonymous, "POST", url, map[string]any{"name": "x", "score": 100}, 401)

	alice := cookieClient()
	register(t, alice, server.URL)
	var table HashFieldTable
	json.Unmarshal(request(t, alice, "GET", url, nil, 200), &table)
	if len(table.Items) != 2 || table.Items[0] != (HashFieldEntry{1, "Michael Karbowiak", 97909}) || table.Items[1] != (HashFieldEntry{2, "Peter Petermann", 22831}) {
		t.Fatalf("unexpected seeded table: %+v", table)
	}

	for _, bad := range []map[string]any{
		{"name": "Alice", "score": 0},
		{"name": "Alice", "score": -10},
		{"name": "Alice", "score": 105},
		{"name": "Alice", "score": 10_000_010},
		{"name": "   ", "score": 100},
		{"name": strings.Repeat("a", 25), "score": 100},
		{"name": "bad\u0007bell", "score": 100},
		{"name": "right‮left", "score": 100},
	} {
		request(t, alice, "POST", url, bad, 422)
	}

	var placed HashFieldResult
	json.Unmarshal(request(t, alice, "POST", url, map[string]any{"name": "  Alice \t Liddell ", "score": 30000}, 200), &placed)
	if !placed.Placed || placed.Rank != 2 || placed.Items[1] != (HashFieldEntry{2, "Alice Liddell", 30000}) || len(placed.Items) != 3 {
		t.Fatalf("score did not place: %+v", placed)
	}
	// One account cannot submit again straight away.
	request(t, alice, "POST", url, map[string]any{"name": "Alice", "score": 40000}, 429)

	// Another account sees the same table.
	bob := cookieClient()
	register(t, bob, server.URL)
	json.Unmarshal(request(t, bob, "GET", url, nil, 200), &table)
	if len(table.Items) != 3 || table.Items[1].Name != "Alice Liddell" {
		t.Fatalf("table is not shared: %+v", table)
	}
}

func TestHashFieldLimiterSpacesAndCapsSubmissions(t *testing.T) {
	var l hashFieldLimiter
	now := time.Unix(1_000_000, 0)
	if !l.allow("a", now) || l.allow("a", now.Add(hashFieldGap-time.Second)) {
		t.Fatal("submissions closer than the gap were allowed")
	}
	if !l.allow("b", now) {
		t.Fatal("limits are not per account")
	}
	at := now
	for i := 1; i < hashFieldHourly; i++ {
		at = at.Add(hashFieldGap)
		if !l.allow("a", at) {
			t.Fatal("spaced submission refused", i)
		}
	}
	if l.allow("a", at.Add(hashFieldGap)) {
		t.Fatal("hourly cap not applied")
	}
	if !l.allow("a", now.Add(time.Hour+time.Second)) {
		t.Fatal("cap did not expire after an hour")
	}
}
