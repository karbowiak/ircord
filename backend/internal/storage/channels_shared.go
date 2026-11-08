package storage

import (
	"encoding/json"
	"time"
)

// SharedChannel is one stored row of channel-wide settings.
type SharedChannel struct {
	Network string
	Channel string
	Payload json.RawMessage
}

// SharedChannels returns every stored channel-wide settings row.
func (s *Store) SharedChannels() ([]SharedChannel, error) {
	rows, err := s.DB.Query("SELECT network, channel, payload FROM channel_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SharedChannel{}
	for rows.Next() {
		var v SharedChannel
		var raw []byte
		if err := rows.Scan(&v.Network, &v.Channel, &raw); err != nil {
			return nil, err
		}
		v.Payload = raw
		result = append(result, v)
	}
	return result, rows.Err()
}

// SaveSharedChannel stores the channel-wide settings for one upstream channel.
func (s *Store) SaveSharedChannel(network, channel string, payload json.RawMessage) error {
	_, err := s.DB.Exec("INSERT INTO channel_settings(network, channel, payload, updated_ns) VALUES(?,?,?,?) ON CONFLICT(network, channel) DO UPDATE SET payload=excluded.payload, updated_ns=excluded.updated_ns", network, channel, []byte(payload), time.Now().UnixNano())
	return err
}
