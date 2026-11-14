package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"ircord/backend/internal/model"
	"ircord/backend/internal/notify"
)

func (s *Store) Commit(owner, kind string, data any) (model.Event, error) {
	tx, err := s.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return model.Event{}, err
	}
	defer tx.Rollback()
	raw, err := json.Marshal(data)
	if err != nil {
		return model.Event{}, err
	}
	var seq uint64
	if err = tx.QueryRow("UPDATE accounts SET seq=seq+1 WHERE id=? AND deleted=0 RETURNING seq", owner).Scan(&seq); err != nil {
		return model.Event{}, err
	}
	entityKind, entityID := "", ""
	switch v := data.(type) {
	case model.Invitation:
		if v.Party != nil {
			break
		}
		_, err = tx.Exec("INSERT INTO invitations(owner,network,channel_key,id,channel,inviter_id,inviter_nick,created_ns,rejected) VALUES(?,?,?,?,?,?,?,?,0) ON CONFLICT(owner,network,channel_key) DO NOTHING", owner, v.NetworkID, v.ChannelKey, v.ID, v.Channel, v.InviterID, v.InviterNick, v.CreatedAt.UnixNano())
		if err != nil {
			return model.Event{}, err
		}
	case model.InvitationRemoval:
		if v.Rejected {
			_, err = tx.Exec("UPDATE invitations SET rejected=1 WHERE owner=? AND id=?", owner, v.ID)
		} else {
			_, err = tx.Exec("DELETE FROM invitations WHERE owner=? AND id=? AND rejected=0", owner, v.ID)
		}
		if err != nil {
			return model.Event{}, err
		}
	case model.Removal:
		if kind == "network.removed" {
			if _, err = tx.Exec("DELETE FROM invitations WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			// A conversation closed earlier has no entity left, but its ID still
			// names the network: the network ID, "_", then 24 hex digits.
			for _, table := range []string{"messages", "outbox", "history_state", "conversation_read", "reactions", "message_mentions", "message_links"} {
				if _, err = tx.Exec("DELETE FROM "+table+" WHERE owner=? AND (conversation IN (SELECT id FROM entities WHERE owner=? AND kind='conversation' AND json_extract(payload,'$.networkId')=?) OR (length(conversation)=length(?)+25 AND substr(conversation,1,length(?)+1)=?||'_' AND substr(conversation,length(?)+2) NOT GLOB '*[^0-9a-f]*'))", owner, owner, v.ID, v.ID, v.ID, v.ID, v.ID); err != nil {
					return model.Event{}, err
				}
			}
			// Bookmarks name their network directly, so ones into conversations closed
			// earlier (whose entity is already gone) are removed too.
			if _, err = tx.Exec("DELETE FROM saved_messages WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM inbox_items WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM entities WHERE owner=? AND ((kind='conversation' AND json_extract(payload,'$.networkId')=?) OR (kind IN ('network','connection') AND id=?) OR (kind='member' AND substr(id,1,length(?)+1)=?||':'))", owner, v.ID, v.ID, v.ID, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM channel_cache_meta WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM networks WHERE owner=? AND id=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM app_data WHERE owner=? AND kind IN (?,?)", owner, profilePublishedKind(v.ID), profileImportedKind(v.ID)); err != nil {
				return model.Event{}, err
			}
			if _, err = tx.Exec("DELETE FROM server_lines WHERE owner=? AND network=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
			// Previous replay frames can contain this network's history. A stale device
			// must resnapshot after deletion instead of replaying the removed content.
			if _, err = tx.Exec("DELETE FROM events WHERE owner=?", owner); err != nil {
				return model.Event{}, err
			}
		} else if kind == "conversation.removed" {
			if _, err = tx.Exec("DELETE FROM entities WHERE owner=? AND kind='conversation' AND id=?", owner, v.ID); err != nil {
				return model.Event{}, err
			}
		}

	case model.SavedMessage:
		_, err = tx.Exec("INSERT INTO saved_messages(owner,message,conversation,network,saved_ns) VALUES(?,?,?,?,?) ON CONFLICT(owner,message) DO NOTHING", owner, v.MessageID, v.ConversationID, v.NetworkID, v.SavedAt.UnixNano())
		if err != nil {
			return model.Event{}, err
		}
	case model.InboxEntry:
		_, err = tx.Exec("INSERT INTO inbox_items(owner,message,conversation,network,sender,reasons,created_ns) VALUES(?,?,?,?,?,?,?) ON CONFLICT(owner,message) DO UPDATE SET reasons=reasons|excluded.reasons", owner, v.MessageID, v.ConversationID, v.NetworkID, v.SenderID, inboxBits(v.Reasons), v.CreatedAt.UnixNano())
		if err == nil {
			err = pruneInbox(tx, owner, time.Now())
		}
		if err != nil {
			return model.Event{}, err
		}
	case model.InboxChange:
		dismissed := sql.NullInt64{}
		if v.Dismissed {
			dismissed = sql.NullInt64{Int64: time.Now().UnixNano(), Valid: true}
		}
		if _, err = tx.Exec("UPDATE inbox_items SET dismissed_ns=? WHERE owner=? AND message=?", dismissed, owner, v.MessageID); err != nil {
			return model.Event{}, err
		}
	case model.SavedRemoval:
		if _, err = tx.Exec("DELETE FROM saved_messages WHERE owner=? AND message=?", owner, v.MessageID); err != nil {
			return model.Event{}, err
		}
	case model.Member:
		entityKind = "member"
		entityID = v.ID
		if v.Ignored {
			_, err = tx.Exec("INSERT INTO ignored_senders(owner,member) VALUES(?,?) ON CONFLICT DO NOTHING", owner, v.ID)
		} else {
			_, err = tx.Exec("DELETE FROM ignored_senders WHERE owner=? AND member=?", owner, v.ID)
		}
		if err != nil {
			return model.Event{}, err
		}
	case model.ConnectionInfo:
		entityKind = "connection"
		entityID = v.NetworkID
	case model.ServerLogBatch:
		var base int64
		if err = tx.QueryRow("SELECT coalesce(max(seq),0) FROM server_lines WHERE owner=? AND network=?", owner, v.NetworkID).Scan(&base); err != nil {
			return model.Event{}, err
		}
		for i, line := range v.Lines {
			payload, _ := json.Marshal(line)
			if _, err = tx.Exec("INSERT INTO server_lines(owner,network,id,seq,payload) VALUES(?,?,?,?,?) ON CONFLICT(owner,network,id) DO NOTHING", owner, v.NetworkID, line.ID, base+int64(i)+1, payload); err != nil {
				return model.Event{}, err
			}
		}
		if err = pruneServerLines(tx, owner, v.NetworkID); err != nil {
			return model.Event{}, err
		}
	case model.ServerLine:
		var logSeq int64
		if err = tx.QueryRow("SELECT coalesce(max(seq),0)+1 FROM server_lines WHERE owner=? AND network=?", owner, v.NetworkID).Scan(&logSeq); err != nil {
			return model.Event{}, err
		}
		if _, err = tx.Exec("INSERT INTO server_lines(owner,network,id,seq,payload) VALUES(?,?,?,?,?)", owner, v.NetworkID, v.ID, logSeq, raw); err != nil {
			return model.Event{}, err
		}
		if err = pruneServerLines(tx, owner, v.NetworkID); err != nil {
			return model.Event{}, err
		}
	case model.Network:
		entityKind = "network"
		entityID = v.ID
	case model.Conversation:
		entityKind = "conversation"
		entityID = v.ID
	case model.Preferences:
		entityKind = "preferences"
		entityID = "self"
	case model.Message:
		if err = positionMessage(tx, owner, &v); err != nil {
			return model.Event{}, err
		}
		raw, _ = json.Marshal(v)
		if !v.Historical && v.UpstreamID != "" && (v.Kind == "message" || v.Kind == "action" || v.Kind == "notice") {
			if _, err = tx.Exec("INSERT INTO history_state(owner,conversation,synced_id,synced_ns,gap_open) VALUES(?,?,?,?,0) ON CONFLICT(owner,conversation) DO UPDATE SET synced_id=excluded.synced_id,synced_ns=excluded.synced_ns WHERE gap_open=0 AND excluded.synced_ns>=synced_ns", owner, v.ConversationID, v.UpstreamID, v.CreatedAt.UnixNano()); err != nil {
				return model.Event{}, err
			}
		}
		if err = putMessage(tx, owner, v); err != nil {
			return model.Event{}, err
		}
		if v.ClientID != "" {
			state := "sent"
			if v.UpstreamID == "" {
				state = "unconfirmed"
			}
			if _, err = tx.Exec("UPDATE outbox SET state=?,message_id=? WHERE owner=? AND client_id=?", state, v.ID, owner, v.ClientID); err != nil {
				return model.Event{}, err
			}
		}
	}
	if entityKind != "" {
		entityRaw := raw
		if conv, ok := data.(model.Conversation); ok && conv.ChannelIdentity != nil {
			// Keep the last verified route with its existing account-owned entity.
			// Private event frames and HTTP responses retain the public DTO only.
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(raw, &fields); err != nil {
				return model.Event{}, err
			}
			fields["_channelIdentity"], err = json.Marshal(conv.ChannelIdentity)
			if err != nil {
				return model.Event{}, err
			}
			entityRaw, err = json.Marshal(fields)
			if err != nil {
				return model.Event{}, err
			}
		}
		if _, err = tx.Exec("INSERT INTO entities(owner,kind,id,payload) VALUES(?,?,?,?) ON CONFLICT(owner,kind,id) DO UPDATE SET payload=excluded.payload", owner, entityKind, entityID, entityRaw); err != nil {
			return model.Event{}, err
		}
	}
	if kind == "notification-settings.updated" {
		if err = syncIgnored(tx, owner, raw); err != nil {
			return model.Event{}, err
		}
	}
	if kind == "appearance.updated" || kind == "library.updated" || (kind == "client-preferences.updated" || kind == "notification-settings.updated") {
		if _, err = tx.Exec("INSERT INTO app_data(owner,kind,payload,updated_ns) VALUES(?,?,?,?) ON CONFLICT(owner,kind) DO UPDATE SET payload=excluded.payload,updated_ns=excluded.updated_ns", owner, kind, raw, time.Now().UnixNano()); err != nil {
			return model.Event{}, err
		}
	}
	eventData := raw
	if batch, ok := data.(model.ServerLogBatch); ok {
		eventData, _ = json.Marshal(map[string]string{"networkId": batch.NetworkID})
	}
	if kind == "appearance.updated" || kind == "library.updated" || kind == "client-preferences.updated" || kind == "notification-settings.updated" {
		sum := sha256.Sum256(raw)
		eventData, _ = json.Marshal(map[string]string{"revision": hex.EncodeToString(sum[:])})
	}
	event := model.Event{V: 1, Seq: seq, Type: kind, Data: eventData}
	eventRaw, _ := json.Marshal(event)
	if _, err = tx.Exec("INSERT INTO events(owner,seq,created_ns,payload) VALUES(?,?,?,?)", owner, seq, time.Now().UnixNano(), eventRaw); err != nil {
		return model.Event{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.Event{}, err
	}
	return event, nil
}

func (s *Store) Replay(owner string, after uint64, limit int) ([]model.Event, bool, error) {
	var head uint64
	if err := s.DB.QueryRow("SELECT seq FROM accounts WHERE id=?", owner).Scan(&head); err != nil {
		return nil, false, err
	}
	var oldest sql.NullInt64
	if err := s.DB.QueryRow("SELECT MIN(seq) FROM events WHERE owner=?", owner).Scan(&oldest); err != nil {
		return nil, false, err
	}
	if after > head || (!oldest.Valid && after != head) || (oldest.Valid && after+1 < uint64(oldest.Int64)) {
		return nil, false, nil
	}
	rows, err := s.DB.Query("SELECT payload FROM events WHERE owner=? AND seq>? ORDER BY seq LIMIT ?", owner, after, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	events := []model.Event{}
	for rows.Next() {
		var raw []byte
		var e model.Event
		if err = rows.Scan(&raw); err != nil {
			return nil, false, err
		}
		if err = json.Unmarshal(raw, &e); err != nil {
			return nil, false, err
		}
		events = append(events, e)
	}
	if len(events) > limit {
		return nil, false, nil
	}
	return events, true, rows.Err()
}

func (s *Store) Head(owner string) (uint64, error) {
	var n uint64
	err := s.DB.QueryRow("SELECT seq FROM accounts WHERE id=?", owner).Scan(&n)
	return n, err
}

func (s *Store) PruneEvents(before time.Time) error {
	_, err := s.DB.Exec("DELETE FROM events WHERE created_ns<?", before.UnixNano())
	return err
}

func syncIgnored(tx *sql.Tx, owner string, raw []byte) error {
	policy := notify.Compile(raw)
	rows, err := tx.Query("SELECT payload FROM entities WHERE owner=? AND kind='member'", owner)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			rows.Close()
			return err
		}
		var member model.Member
		if err = json.Unmarshal(data, &member); err != nil {
			rows.Close()
			return err
		}
		if member.ID != owner && policy.Ignored(member.Nick, member.Username, member.Host) {
			ids = append(ids, member.ID)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM ignored_senders WHERE owner=?", owner); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err = tx.Exec("INSERT INTO ignored_senders(owner,member) VALUES(?,?)", owner, id); err != nil {
			return err
		}
	}
	return nil
}
