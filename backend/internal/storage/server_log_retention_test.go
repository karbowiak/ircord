package storage

import (
	"fmt"
	"testing"
	"time"

	"ircord/backend/internal/model"
)

func TestServerLogRetentionOwnershipAndDuplicateBehavior(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	account(t, s, "alice")
	account(t, s, "bob")

	line := func(id, network, text string) model.ServerLine {
		return model.ServerLine{ID: id, NetworkID: network, Time: time.Unix(1, 0).UTC(), Kind: "notice", Command: "NOTICE", Params: []string{}, Text: text}
	}
	commitLine := func(owner string, value model.ServerLine) error {
		_, err := s.Commit(owner, "server.line", value)
		return err
	}
	readLines := func(owner, network string) []model.ServerLine {
		t.Helper()
		page, err := s.ServerLog(owner, network, "", 1000)
		if err != nil {
			t.Fatal(err)
		}
		return page.Items
	}
	assertRetainedRange := func(owner, network, prefix string) []model.ServerLine {
		t.Helper()
		items := readLines(owner, network)
		if len(items) != 500 {
			t.Fatalf("%s/%s retained %d rows; want 500", owner, network, len(items))
		}
		if items[0].ID != prefix+"001" || items[len(items)-1].ID != prefix+"500" {
			t.Fatalf("%s/%s retained range = %q..%q; want %q001..%q500", owner, network, items[0].ID, items[len(items)-1].ID, prefix, prefix)
		}
		return items
	}

	// These rows must survive retention in other owners' or networks' log streams.
	if err := commitLine("bob", line("keep-bob", "single", "bob")); err != nil {
		t.Fatal(err)
	}
	if err := commitLine("alice", line("keep-other", "other", "other network")); err != nil {
		t.Fatal(err)
	}

	for i := 0; i <= 500; i++ {
		id := fmt.Sprintf("single-%03d", i)
		if err := commitLine("alice", line(id, "single", "original-"+id)); err != nil {
			t.Fatalf("single insert %s: %v", id, err)
		}
	}
	single := assertRetainedRange("alice", "single", "single-")

	// Single inserts retain their original conflict behavior: a duplicate ID fails
	// and the enclosing account sequence increment rolls back with the transaction.
	headBefore, err := s.Head("alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := commitLine("alice", line("single-500", "single", "replacement")); err == nil {
		t.Fatal("single duplicate insert unexpectedly succeeded")
	}
	headAfter, err := s.Head("alice")
	if err != nil || headAfter != headBefore {
		t.Fatalf("duplicate single insert advanced head from %d to %d: %v", headBefore, headAfter, err)
	}
	if single[len(single)-1].Text != "original-single-500" {
		t.Fatalf("single duplicate changed stored line: %+v", single[len(single)-1])
	}

	// Batch inserts still ignore duplicate IDs, retain the first payload, and keep
	// the same 500-line bound as the single insert path.
	batch := make([]model.ServerLine, 0, 502)
	for i := 0; i <= 500; i++ {
		id := fmt.Sprintf("batch-%03d", i)
		batch = append(batch, line(id, "batch", "original-"+id))
	}
	batch = append(batch, line("batch-001", "batch", "replacement"))
	if _, err := s.Commit("alice", "server.log.batch", model.ServerLogBatch{NetworkID: "batch", Lines: batch}); err != nil {
		t.Fatal(err)
	}
	batchItems := assertRetainedRange("alice", "batch", "batch-")
	if batchItems[0].Text != "original-batch-001" {
		t.Fatalf("batch duplicate replaced stored payload: %+v", batchItems[0])
	}

	if got := readLines("bob", "single"); len(got) != 1 || got[0].ID != "keep-bob" {
		t.Fatalf("alice retention affected bob's network logs: %+v", got)
	}
	if got := readLines("alice", "other"); len(got) != 1 || got[0].ID != "keep-other" {
		t.Fatalf("alice retention affected another network: %+v", got)
	}
}
