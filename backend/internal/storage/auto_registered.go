package storage

import "time"

// SetAutoRegistered records, or with registered false forgets, that IRCord registered a
// channel conversation with ChanServ because the account created it.
func (s *Store) SetAutoRegistered(owner, conversation string, registered bool) error {
	if !registered {
		_, err := s.DB.Exec("DELETE FROM auto_registered_channels WHERE owner=? AND conversation=?", owner, conversation)
		return err
	}
	_, err := s.DB.Exec("INSERT INTO auto_registered_channels(owner, conversation, registered_ns) VALUES(?,?,?) ON CONFLICT(owner, conversation) DO UPDATE SET registered_ns=excluded.registered_ns", owner, conversation, time.Now().UnixNano())
	return err
}

// AutoRegistered reports whether IRCord registered the channel conversation automatically.
func (s *Store) AutoRegistered(owner, conversation string) (bool, error) {
	var found bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM auto_registered_channels WHERE owner=? AND conversation=?)", owner, conversation).Scan(&found)
	return found, err
}
