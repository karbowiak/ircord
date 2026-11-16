package storage

import "strings"

// ReferencedMembers returns the member IDs an account's retained messages,
// reactions and inbox items still name, so pruning can keep them.
func (s *Store) ReferencedMembers(owner string) (map[string]bool, error) {
	// One pass over messages yields both the sender and an event's target.
	rows, err := s.read.Query(`SELECT sender, CASE WHEN kind='event' THEN json_extract(payload,'$.event.targetId') END FROM messages WHERE owner=?
		UNION ALL SELECT sender, NULL FROM reactions WHERE owner=?
		UNION ALL SELECT sender, NULL FROM inbox_items WHERE owner=?`, owner, owner, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var sender, target *string
		if err = rows.Scan(&sender, &target); err != nil {
			return nil, err
		}
		for _, id := range []*string{sender, target} {
			if id != nil && *id != "" {
				out[*id] = true
			}
		}
	}
	return out, rows.Err()
}

// DeleteMembers removes stored members in short transactions.
func (s *Store) DeleteMembers(owner string, ids []string) error {
	for len(ids) > 0 {
		chunk := ids[:min(len(ids), deleteChunkRows)]
		ids = ids[len(chunk):]
		args := []any{owner}
		for _, id := range chunk {
			args = append(args, id)
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		if _, err := s.DB.Exec("DELETE FROM entities WHERE owner=? AND kind='member' AND id IN ("+placeholders+")", args...); err != nil {
			return err
		}
	}
	return nil
}
