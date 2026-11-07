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

func TestLegacyInvitesBecomePrivateStateWithoutLosingHistory(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	account(t, s, "alice")
	_, _ = s.Commit("alice", "network.updated", model.Network{ID: "net"})
	_, _ = s.Commit("alice", "member.updated", model.Member{ID: "peer", Nick: "Peer"})
	for _, room := range []string{"pending", "actual"} {
		_, err = s.Commit("alice", "conversation.updated", model.Conversation{ID: room, NetworkID: "net", Kind: "channel", Name: room, Target: "#" + room})
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Commit("alice", "message.created", model.Message{ID: room + "_invite", ConversationID: room, Kind: "event", SenderID: "peer", CreatedAt: time.Now(), Event: &model.MembershipEvent{Type: "invite"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err = s.Commit("alice", "message.created", model.Message{ID: "real", ConversationID: "actual", Kind: "message", Text: "keep this channel", CreatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.Head("alice")
	if _, err = s.DB.Exec("DELETE FROM goose_db_version WHERE version_id=4"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	invites, err := s.Invitations("alice")
	if err != nil || len(invites) != 1 || invites[0].Channel != "#pending" || invites[0].InviterNick != "Peer" {
		t.Fatal(invites, err)
	}
	entities, err := s.Entities("alice")
	if err != nil || len(entities["conversation"]) != 1 {
		t.Fatal("real channel lost or placeholder remained", entities, err)
	}
	page, err := s.History("alice", "pending", "", 40)
	if err != nil || len(page.Items) != 1 {
		t.Fatal("canonical history lost", err)
	}
	events, valid, err := s.Replay("alice", before, 100)
	if err != nil || !valid || len(events) != 2 || events[0].Type != "invitation.received" || events[1].Type != "conversation.removed" {
		t.Fatal("devices cannot resynchronize migration", events, valid, err)
	}
}
