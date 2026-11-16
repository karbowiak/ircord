package storage

import (
	"encoding/json"
	"errors"

	"ircord/backend/internal/model"
)

func (s *Store) SaveNetwork(owner string, cfg model.NetworkConfig, enabled bool) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	encrypted := s.seal(owner, cfg.ID, raw)
	_, err = s.DB.Exec("INSERT INTO networks(owner,id,config,enabled) VALUES(?,?,?,?) ON CONFLICT(owner,id) DO UPDATE SET config=excluded.config,enabled=excluded.enabled,signed_out=CASE WHEN excluded.enabled THEN 0 ELSE networks.signed_out END", owner, cfg.ID, encrypted, enabled)
	return err
}

// SignOutNetwork turns off a network the account was connected to when it signed out of
// its last device. SignedOutNetworks lists them for the next sign-in; enabling the network
// again with SaveNetwork clears the mark.
func (s *Store) SignOutNetwork(owner, id string) error {
	_, err := s.DB.Exec("UPDATE networks SET enabled=0,signed_out=1 WHERE owner=? AND id=?", owner, id)
	return err
}

// SignOutEnabledNetworks turns off every network of the owner that is on, connected or not
// (a full connection pool can leave one on without a connection), and marks them signed out.
func (s *Store) SignOutEnabledNetworks(owner string) error {
	_, err := s.DB.Exec("UPDATE networks SET enabled=0,signed_out=1 WHERE owner=? AND enabled=1", owner)
	return err
}

// SignedOutNetworks lists the IDs of the owner's networks turned off by SignOutNetwork.
func (s *Store) SignedOutNetworks(owner string) ([]string, error) {
	rows, err := s.DB.Query("SELECT id FROM networks WHERE owner=? AND signed_out=1 ORDER BY id", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type SavedNetwork struct {
	Owner   string
	Config  model.NetworkConfig
	Enabled bool
}

func (s *Store) Networks(owner string) ([]SavedNetwork, error) {
	query := "SELECT owner,id,config,enabled FROM networks"
	args := []any{}
	if owner != "" {
		query += " WHERE owner=?"
		args = append(args, owner)
	}
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SavedNetwork{}
	for rows.Next() {
		var owner, id string
		var raw []byte
		var enabled bool
		if err = rows.Scan(&owner, &id, &raw, &enabled); err != nil {
			return nil, err
		}
		raw, err = s.open(owner, id, raw)
		if err != nil {
			return nil, errors.New("cannot decrypt network configuration: check master.key")
		}
		var cfg model.NetworkConfig
		if err = json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		result = append(result, SavedNetwork{owner, cfg, enabled})
	}
	return result, rows.Err()
}

// ClearNetworkRows removes a network's history and server window in short
// transactions before its network.removed commit, so removing a network with
// years of history does not hold the single writer and stall other accounts.
// The caller has disconnected the network and holds the account's workspace
// lock, so nothing writes new rows for it meanwhile. The commit removes the rest.
func (s *Store) ClearNetworkRows(owner, network string) error {
	// The same conversation predicates as the network.removed commit: open
	// conversations by their entity, closed ones by their ID shape.
	open := "owner=? AND conversation IN (SELECT id FROM entities WHERE owner=? AND kind='conversation' AND json_extract(payload,'$.networkId')=?)"
	closed := "owner=? AND conversation > ?||'_' AND conversation < ?||'`' AND length(conversation)=length(?)+25 AND substr(conversation,length(?)+2) NOT GLOB '*[^0-9a-f]*'"
	for _, table := range []string{"messages", "message_mentions", "message_links", "reactions", "outbox", "history_state", "conversation_read"} {
		if err := s.deleteInChunks(table, open, owner, owner, network); err != nil {
			return err
		}
		if err := s.deleteInChunks(table, closed, owner, network, network, network, network); err != nil {
			return err
		}
	}
	for _, table := range []string{"saved_messages", "inbox_items", "server_lines"} {
		if err := s.deleteInChunks(table, "owner=? AND network=?", owner, network); err != nil {
			return err
		}
	}
	return nil
}
