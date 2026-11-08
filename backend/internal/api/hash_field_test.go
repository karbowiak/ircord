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

	for _, bad := range []struct {
		name   string
		score  int
		detail string
	}{
		{"Alice", 0, "score"},
		{"Alice", -10, "score"},
		{"Alice", 105, "score"},
		{"Alice", 10_000_010, "score"},
		{"   ", 100, hashFieldNameLength},
		{strings.Repeat("a", 25), 100, hashFieldNameLength},
		{"bad\u0007bell", 100, hashFieldNameCharacters},
		{"right\u202eleft", 100, hashFieldNameCharacters},
		{"private\ue000use", 100, hashFieldNameCharacters},
		{"non\uffffchar", 100, hashFieldNameCharacters},
	} {
		body := request(t, alice, "POST", url, map[string]any{"name": bad.name, "score": bad.score}, 422)
		if !strings.Contains(string(body), bad.detail) {
			t.Fatalf("%q/%d: unexpected problem %s", bad.name, bad.score, body)
		}
	}

	var placed HashFieldResult
	json.Unmarshal(request(t, alice, "POST", url, map[string]any{"name": "  Alice \t Liddell ", "score": 30000}, 200), &placed)
	if !placed.Placed || placed.Rank != 2 || placed.Items[1] != (HashFieldEntry{2, "Alice Liddell", 30000}) || len(placed.Items) != 3 {
		t.Fatalf("score did not place: %+v", placed)
	}
	// A lone surrogate (half an emoji) decodes to U+FFFD and is refused, not mistaken for a length.
	lone := json.RawMessage(`{"name":"` + strings.Repeat("a", 23) + `\ud83d","score":100}`)
	if body := request(t, alice, "POST", url, lone, 422); !strings.Contains(string(body), hashFieldNameCharacters) {
		t.Fatalf("lone surrogate: unexpected problem %s", body)
	}
	if name, problem := hashFieldName("👨‍👩‍👧 " + strings.Repeat("é", 10)); problem != "" || name != "👨‍👩‍👧 "+strings.Repeat("é", 10) {
		t.Fatal("emoji sequences and accented letters are refused", problem)
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
