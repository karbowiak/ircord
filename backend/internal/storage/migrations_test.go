package storage

import (
	"database/sql"
	"ircord/backend/internal/model"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLegacyMigrationPreservesStateAndBacksUpOnce(t *testing.T) {
	for _, position := range []bool{true, false} {
		t.Run(map[bool]string{true: "with-position", false: "without-position"}[position], func(t *testing.T) {
			dir := t.TempDir()
			s, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			account(t, s, "alice")
			cfg := model.NetworkConfig{ID: "net", Nick: "Alice", ServerPassword: "migration-test-credential"}
			if err = s.SaveNetwork("alice", cfg, true); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Commit("alice", "profile.updated", model.Member{ID: "alice", Nick: "Alice"}); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Commit("alice", "message.created", model.Message{ID: "message", ConversationID: "channel", Kind: "message", Text: "retained history", CreatedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
			if _, err = s.DB.Exec("DROP TABLE goose_db_version"); err != nil {
				t.Fatal(err)
			}
			if !position {
				if _, err = s.DB.Exec("DROP INDEX messages_position; ALTER TABLE messages DROP COLUMN position"); err != nil {
					t.Fatal(err)
				}
			}
			s.Close()
			s, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			nets, err := s.Networks("alice")
			if err != nil || len(nets) != 1 || nets[0].Config.ServerPassword != cfg.ServerPassword {
				t.Fatal("encrypted credentials changed", err)
			}
			page, err := s.History("alice", "channel", "", 40)
			if err != nil || len(page.Items) != 1 || page.Items[0].Text != "retained history" {
				t.Fatal("history changed", err)
			}
			s.Close()
			s, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			s.Close()
			backups, _ := filepath.Glob(filepath.Join(dir, "pre-goose-*.db"))
			if len(backups) != 1 {
				t.Fatal("expected one legacy backup", len(backups))
			}
			info, _ := os.Stat(backups[0])
			if info.Mode().Perm() != 0600 {
				t.Fatal("backup is not private")
			}
			db, err := sql.Open("sqlite", backups[0])
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var count int
			if err = db.QueryRow("SELECT count(*) FROM messages").Scan(&count); err != nil || count != 1 {
				t.Fatal("backup not readable", err)
			}
		})
	}
}

func TestRefuseFutureSchema(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("INSERT INTO goose_db_version(version_id,is_applied) VALUES(999,1)"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s, err = Open(dir); err == nil {
		s.Close()
		t.Fatal("downgrade opened a future database")
	}
}
