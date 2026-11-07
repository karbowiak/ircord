package storage

import (
	"database/sql"
	"errors"
	"ircord/backend/internal/model"
	"strings"
	"time"
)

func (s *Store) SaveChannels(owner, network string, items []model.ChannelListing, started time.Time) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM channel_cache_meta WHERE owner=? AND network=?", owner, network); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO channel_cache_meta(owner,network,at_ns) VALUES(?,?,?)", owner, network, started.UnixNano()); err != nil {
		return err
	}
	insert, err := tx.Prepare("INSERT INTO channel_cache(owner,network,name,users,topic,search) VALUES(?,?,?,?,?,?) ON CONFLICT(owner,network,name) DO NOTHING")
	if err != nil {
		return err
	}
	defer insert.Close()
	for _, v := range items {
		if _, err = insert.Exec(owner, network, v.Name, v.Users, v.Topic, strings.ToLower(v.Name+" "+v.Topic)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) CachedChannels(owner, network, q string, limit int, since time.Time) ([]model.ChannelListing, bool, error) {
	var at int64
	err := s.DB.QueryRow("SELECT at_ns FROM channel_cache_meta WHERE owner=? AND network=?", owner, network).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if time.Since(time.Unix(0, at)) > 3*time.Minute || at < since.UnixNano() {
		return nil, false, nil
	}
	rows, err := s.DB.Query("SELECT name,users,topic FROM channel_cache WHERE owner=? AND network=? AND instr(search,?)>0 ORDER BY users DESC,name LIMIT ?", owner, network, strings.ToLower(q), limit)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	items := []model.ChannelListing{}
	for rows.Next() {
		var v model.ChannelListing
		if err = rows.Scan(&v.Name, &v.Users, &v.Topic); err != nil {
			return nil, false, err
		}
		items = append(items, v)
	}
	return items, true, rows.Err()
}
func (s *Store) InvalidateChannels(owner, network string) error {
	_, err := s.DB.Exec("DELETE FROM channel_cache_meta WHERE owner=? AND network=?", owner, network)
	return err
}
func (s *Store) PruneChannelCache(before time.Time) error {
	_, err := s.DB.Exec("DELETE FROM channel_cache_meta WHERE at_ns<?", before.UnixNano())
	return err
}
