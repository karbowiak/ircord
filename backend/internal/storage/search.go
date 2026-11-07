package storage

import (
	"encoding/json"
	"ircord/backend/internal/model"
)

func (s *Store) Search(owner, query, conversation string, limit int) ([]model.Message, error) {
	rows, err := s.DB.Query("SELECT payload FROM messages WHERE owner=? AND (?='' OR conversation=?) AND instr(lower(text),lower(?))>0 ORDER BY created_ns DESC,id DESC LIMIT ?", owner, conversation, conversation, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Message{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var msg model.Message
		if err = json.Unmarshal(raw, &msg); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	return out, rows.Err()
}
