package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/lrstanley/girc"
	"ircord/backend/internal/model"
	"strings"
	"time"
)

// Convert only the old invite-only placeholders. Keep history and every channel
// that has actual membership or conversation activity.
func upgradeLegacyInvitations(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT e.owner,e.payload FROM entities e WHERE e.kind='conversation' AND json_extract(e.payload,'$.kind')='channel' AND coalesce(json_extract(e.payload,'$.joined'),0)=0 AND EXISTS(SELECT 1 FROM messages m WHERE m.owner=e.owner AND m.conversation=e.id) AND NOT EXISTS(SELECT 1 FROM messages m WHERE m.owner=e.owner AND m.conversation=e.id AND (m.kind!='event' OR coalesce(json_extract(m.payload,'$.event.type'),'')!='invite'))`)
	if err != nil {
		return err
	}
	type legacy struct {
		owner        string
		conversation model.Conversation
	}
	items := []legacy{}
	for rows.Next() {
		var v legacy
		var raw []byte
		if err = rows.Scan(&v.owner, &raw); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(raw, &v.conversation); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	emit := func(owner, kind string, value any) error {
		var seq uint64
		if err := tx.QueryRowContext(ctx, "UPDATE accounts SET seq=seq+1 WHERE id=? RETURNING seq", owner).Scan(&seq); err != nil {
			return err
		}
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(model.Event{V: 1, Seq: seq, Type: kind, Data: data})
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO events(owner,seq,created_ns,payload) VALUES(?,?,?,?)", owner, seq, time.Now().UnixNano(), raw)
		return err
	}
	for _, entry := range items {
		v := entry.conversation
		var raw []byte
		var msg model.Message
		if err = tx.QueryRowContext(ctx, "SELECT payload FROM messages WHERE owner=? AND conversation=? ORDER BY created_ns DESC,id DESC LIMIT 1", entry.owner, v.ID).Scan(&raw); err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &msg); err != nil {
			return err
		}
		channel := v.Target
		if channel == "" {
			channel = "#" + v.Name
		}
		key := girc.ToRFC1459(channel)
		var network model.Network
		if err = tx.QueryRowContext(ctx, "SELECT payload FROM entities WHERE owner=? AND kind='network' AND id=?", entry.owner, v.NetworkID).Scan(&raw); err == nil {
			if err = json.Unmarshal(raw, &network); err != nil {
				return err
			}
			if network.ISupport["CASEMAPPING"] == "ascii" {
				key = strings.ToLower(channel)
			}
		} else if err != sql.ErrNoRows {
			return err
		}
		inviter := model.Member{Nick: "Someone"}
		if err = tx.QueryRowContext(ctx, "SELECT payload FROM entities WHERE owner=? AND kind='member' AND id=?", entry.owner, msg.SenderID).Scan(&raw); err == nil {
			if err = json.Unmarshal(raw, &inviter); err != nil {
				return err
			}
		} else if err != sql.ErrNoRows {
			return err
		}
		invitation := model.Invitation{ID: "legacy_" + msg.ID, NetworkID: v.NetworkID, Channel: channel, ChannelKey: key, InviterID: msg.SenderID, InviterNick: inviter.Nick, CreatedAt: msg.CreatedAt}
		result, err := tx.ExecContext(ctx, "INSERT INTO invitations(owner,network,channel_key,id,channel,inviter_id,inviter_nick,created_ns,rejected) VALUES(?,?,?,?,?,?,?,?,0) ON CONFLICT(owner,network,channel_key) DO NOTHING", entry.owner, v.NetworkID, key, invitation.ID, channel, msg.SenderID, inviter.Nick, msg.CreatedAt.UnixNano())
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n > 0 {
			if err = emit(entry.owner, "invitation.received", invitation); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM entities WHERE owner=? AND kind='conversation' AND id=?", entry.owner, v.ID); err != nil {
			return err
		}
		if err = emit(entry.owner, "conversation.removed", model.Removal{ID: v.ID}); err != nil {
			return err
		}
	}
	return nil
}
