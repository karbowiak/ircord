package storage

import (
	"strings"
	"testing"
	"time"
)

func TestOfflineHistoryUsesOrderedAccountIndex(t *testing.T) {
	s := catalogStore(t)
	for _, cursor := range []offlineCursor{
		{Since: time.Now().Add(-7 * 24 * time.Hour).UnixNano()},
		{Since: time.Now().Add(-7 * 24 * time.Hour).UnixNano(), At: time.Now().UnixNano(), ID: "boundary"},
	} {
		query, args := offlineHistorySQL("alice", cursor, 500)
		rows, err := s.DB.Query("EXPLAIN QUERY PLAN "+query, args...)
		if err != nil {
			t.Fatal(err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, detail)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(plan, "; ")
		if !strings.Contains(joined, "USING INDEX messages_offline_page") || strings.Contains(joined, "USE TEMP B-TREE") {
			t.Fatalf("archive export must use an ordered account range: %s", joined)
		}
	}
}
