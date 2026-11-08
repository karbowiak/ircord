package storage

import (
	"fmt"
	"testing"
)

func TestHashFieldTableIsSeededKeepsTopTenAndSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	table, err := s.HashFieldScores()
	if err != nil {
		t.Fatal(err)
	}
	if len(table) != 2 || table[0].Name != "Michael Karbowiak" || table[0].Score != 97909 || table[1].Name != "Peter Petermann" || table[1].Score != 22831 {
		t.Fatalf("unexpected seed: %+v", table)
	}
	for i := 1; i <= 8; i++ {
		id, _, err := s.AddHashFieldScore(fmt.Sprintf("player %d", i), int64(i*1000))
		if err != nil || id == 0 {
			t.Fatal("score did not place in a table with room", i, err)
		}
	}
	// The table is full; a score equal to the lowest does not push it out, a better one does.
	if id, table, err := s.AddHashFieldScore("tie", 1000); err != nil || id != 0 || len(table) != HashFieldTableSize {
		t.Fatal("a tie with the lowest entry placed", id, len(table), err)
	}
	id, table, err := s.AddHashFieldScore("new", 25000)
	if err != nil || id == 0 {
		t.Fatal("better score did not place", err)
	}
	if len(table) != HashFieldTableSize || table[1].Name != "new" || table[len(table)-1].Name != "player 2" {
		t.Fatalf("table not re-ranked: %+v", table)
	}
	var stored int
	if err = s.DB.QueryRow("SELECT count(*) FROM hash_field_scores").Scan(&stored); err != nil || stored != HashFieldTableSize {
		t.Fatal("rows outside the table were kept", stored, err)
	}
	s.Close()

	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	after, err := s.HashFieldScores()
	if err != nil || len(after) != HashFieldTableSize || after[1].Name != "new" {
		t.Fatalf("table changed across a restart: %+v %v", after, err)
	}
	// Seeding happens once: the pushed-out entries do not come back.
	for _, entry := range after {
		if entry.Name == "player 1" {
			t.Fatal("dropped entry returned")
		}
	}
}
