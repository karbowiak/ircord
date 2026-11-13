package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"ircord/backend/internal/model"
	"slices"
)

func pruneServerLines(tx *sql.Tx, owner, network string) error {
	_, err := tx.Exec("DELETE FROM server_lines WHERE owner=? AND network=? AND seq<coalesce((SELECT seq FROM server_lines WHERE owner=? AND network=? ORDER BY seq DESC LIMIT 1 OFFSET 499),0)", owner, network, owner, network)
	return err
}

func (s *Store) HasServerLine(owner, network, id string) (bool, error) {
	var found bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM server_lines WHERE owner=? AND network=? AND id=?)", owner, network, id).Scan(&found)
	return found, err
}

// Project retained synthetic history notices into the status window without
// deleting the original history/cursor records. Bound this upgrade per network.
func (s *Store) SystemLogHistory(owner, network string) ([]model.ServerLine, error) {
	rows, err := s.DB.Query(`SELECT m.payload,c.payload FROM messages m JOIN entities c ON c.owner=m.owner AND c.kind='conversation' AND c.id=m.conversation
WHERE m.owner=? AND m.kind='system' AND json_extract(c.payload,'$.networkId')=?
AND NOT EXISTS(SELECT 1 FROM server_lines l WHERE l.owner=m.owner AND l.network=? AND l.id=m.id||':status')
ORDER BY m.created_ns DESC,m.id DESC LIMIT 500`, owner, network, network)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.ServerLine{}
	for rows.Next() {
		var raw, convRaw []byte
		var msg model.Message
		var conv model.Conversation
		if err = rows.Scan(&raw, &convRaw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(convRaw, &conv); err != nil {
			return nil, err
		}
		target := conv.Target
		if target == "" {
			target = "#" + conv.Name
		}
		items = append(items, model.ServerLine{ID: msg.ID + ":status", NetworkID: network, Time: msg.CreatedAt, Kind: "notice", Command: "NOTICE", Source: "HistServ", Params: []string{target, msg.Text}, Text: "[" + target + "] " + msg.Text})
	}
	slices.Reverse(items)
	return items, rows.Err()
}

func (s *Store) ServerLog(owner, network, before string, limit int) (model.ServerLogPage, error) {
	page := model.ServerLogPage{Items: []model.ServerLine{}}
	query := "SELECT payload FROM server_lines WHERE owner=? AND network=?"
	args := []any{owner, network}
	if before != "" {
		var seq uint64
		err := s.DB.QueryRow("SELECT seq FROM server_lines WHERE owner=? AND network=? AND id=?", owner, network, before).Scan(&seq)
		if errors.Is(err, sql.ErrNoRows) {
			return page, ErrNotFound
		}
		if err != nil {
			return page, err
		}
		query += " AND seq<?"
		args = append(args, seq)
	}
	query += " ORDER BY seq DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		var line model.ServerLine
		if err = rows.Scan(&raw); err != nil {
			return page, err
		}
		if err = json.Unmarshal(raw, &line); err != nil {
			return page, err
		}
		page.Items = append(page.Items, line)
	}
	if len(page.Items) > limit {
		page.Items = page.Items[:limit]
		next := page.Items[len(page.Items)-1].ID
		page.NextCursor = &next
	}
	slices.Reverse(page.Items)
	return page, rows.Err()
}
