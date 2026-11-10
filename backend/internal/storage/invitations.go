package storage

import (
	"ircord/backend/internal/model"
	"time"
)

func (s *Store) Invitations(owner string) ([]model.Invitation, error) {
	rows, err := s.DB.Query("SELECT id,network,channel_key,channel,inviter_id,inviter_nick,created_ns,rejected FROM invitations WHERE owner=? ORDER BY created_ns", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []model.Invitation{}
	for rows.Next() {
		var v model.Invitation
		var at int64
		if err = rows.Scan(&v.ID, &v.NetworkID, &v.ChannelKey, &v.Channel, &v.InviterID, &v.InviterNick, &at, &v.Rejected); err != nil {
			return nil, err
		}
		v.CreatedAt = time.Unix(0, at).UTC()
		result = append(result, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	parties, err := s.PendingPartyInvitations(owner)
	return append(result, parties...), err
}
