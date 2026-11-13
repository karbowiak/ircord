package storage

import (
	"database/sql"
	"errors"
	"net/netip"
)

// SaveOrigin retains one verified numeric client address per live account.
func (s *Store) SaveOrigin(owner, address string) error {
	ip, err := netip.ParseAddr(address)
	if err != nil || ip.Zone() != "" {
		return errors.New("invalid client address")
	}
	result, err := s.DB.Exec(`INSERT INTO account_origins(owner,address)
		SELECT id,? FROM accounts WHERE id=? AND deleted=0
		ON CONFLICT(owner) DO UPDATE SET address=excluded.address`, ip.Unmap().String(), owner)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Origin(owner string) (string, error) {
	var address string
	err := s.DB.QueryRow("SELECT address FROM account_origins WHERE owner=?", owner).Scan(&address)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return address, err
}

// ClearOrigins removes retained addresses when WEBIRC is disabled.
func (s *Store) ClearOrigins() error {
	var any bool
	if err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM account_origins)").Scan(&any); err != nil {
		return err
	}
	if !any {
		return nil
	}
	_, err := s.DB.Exec("DELETE FROM account_origins")
	return err
}
