package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"ircord/backend/internal/model"
	"strings"
	"time"
)

// Earlier versions seeded every account with the display name "You" and the settings
// form saved it back as an explicit profile name. Release those placeholders so the
// nickname shows again, and give any other stored self profile an explicit name source.
// Only each account's own profile is touched; other members keep what their network sent.
//
// A self profile reading exactly "You" is reset even when it is marked as a profile
// name: the common way to get that combination was saving the seeded default, and a
// deliberately typed "You" is indistinguishable, so it is reset too (decided in #46).
func upgradeSelfDisplayNames(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT owner,payload FROM entities WHERE kind='member' AND id=owner")
	if err != nil {
		return err
	}
	type self struct {
		owner  string
		member model.Member
	}
	changed := []self{}
	for rows.Next() {
		var v self
		var raw []byte
		if err = rows.Scan(&v.owner, &raw); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(raw, &v.member); err != nil {
			rows.Close()
			return err
		}
		m := &v.member
		switch {
		case m.DisplayName == "You" || strings.TrimSpace(m.DisplayName) == "":
			m.DisplayName, m.NameSource = m.Nick, "nick"
		case m.NameSource != "":
			continue
		case m.DisplayName == m.Nick:
			m.NameSource = "nick"
		default:
			m.NameSource = "profile"
		}
		changed = append(changed, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range changed {
		raw, err := json.Marshal(v.member)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE entities SET payload=? WHERE owner=? AND kind='member' AND id=?", raw, v.owner, v.owner); err != nil {
			return err
		}
		// Devices resuming from an earlier cursor replay the corrected profile.
		var seq uint64
		if err = tx.QueryRowContext(ctx, "UPDATE accounts SET seq=seq+1 WHERE id=? RETURNING seq", v.owner).Scan(&seq); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return err
		}
		event, err := json.Marshal(model.Event{V: 1, Seq: seq, Type: "profile.updated", Data: raw})
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO events(owner,seq,created_ns,payload) VALUES(?,?,?,?)", v.owner, seq, time.Now().UnixNano(), event); err != nil {
			return err
		}
	}
	return nil
}
