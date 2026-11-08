package storage

import (
	"encoding/json"
	"time"
)

// profilePublishedKind is the app_data kind holding what an account's profile last published
// on one network. It is backend state, not a client collection, and is never sent to browsers.
func profilePublishedKind(network string) string { return "profile-published:" + network }

// ProfilePublished returns the draft/metadata-3 values this account last sent for its own
// profile on a network, by key. A missing key was never published from this backend.
func (s *Store) ProfilePublished(owner, network string) (map[string]string, error) {
	raw, err := s.AppData(owner, profilePublishedKind(network))
	if err != nil || raw == nil {
		return map[string]string{}, err
	}
	values := map[string]string{}
	if err = json.Unmarshal(raw, &values); err != nil {
		return map[string]string{}, err
	}
	return values, nil
}

// SaveProfilePublished replaces the values recorded by ProfilePublished.
func (s *Store) SaveProfilePublished(owner, network string, values map[string]string) error {
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("INSERT INTO app_data(owner,kind,payload,updated_ns) VALUES(?,?,?,?) ON CONFLICT(owner,kind) DO UPDATE SET payload=excluded.payload,updated_ns=excluded.updated_ns", owner, profilePublishedKind(network), raw, time.Now().UnixNano())
	return err
}
