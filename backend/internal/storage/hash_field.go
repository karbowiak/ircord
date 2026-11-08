package storage

import (
	"database/sql"
	"time"
)

// HashFieldTableSize is how many entries the Hash Field high score table keeps.
const HashFieldTableSize = 10

// HashFieldScore is one entry of the instance-wide Hash Field high score table.
type HashFieldScore struct {
	ID    int64
	Name  string
	Score int64
}

// HashFieldScores returns the table, best first. Equal scores keep the order they were set in.
func (s *Store) HashFieldScores() ([]HashFieldScore, error) {
	return hashFieldScores(s.DB)
}

type queryer interface {
	Query(string, ...any) (*sql.Rows, error)
}

func hashFieldScores(db queryer) ([]HashFieldScore, error) {
	rows, err := db.Query("SELECT id, name, score FROM hash_field_scores ORDER BY score DESC, id ASC LIMIT ?", HashFieldTableSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []HashFieldScore{}
	for rows.Next() {
		var v HashFieldScore
		if err := rows.Scan(&v.ID, &v.Name, &v.Score); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

// AddHashFieldScore enters a score into the table and drops whatever falls out of the top
// entries. It returns the new entry's id, or 0 when the score did not place, and the table.
// A score must beat an existing entry to push it out: ties keep the earlier entry.
func (s *Store) AddHashFieldScore(name string, score int64) (int64, []HashFieldScore, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, nil, err
	}
	defer tx.Rollback()
	res, err := tx.Exec("INSERT INTO hash_field_scores(name, score, created_ns) VALUES(?,?,?)", name, score, time.Now().UnixNano())
	if err != nil {
		return 0, nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, nil, err
	}
	if _, err = tx.Exec("DELETE FROM hash_field_scores WHERE id NOT IN (SELECT id FROM hash_field_scores ORDER BY score DESC, id ASC LIMIT ?)", HashFieldTableSize); err != nil {
		return 0, nil, err
	}
	table, err := hashFieldScores(tx)
	if err != nil {
		return 0, nil, err
	}
	if err = tx.Commit(); err != nil {
		return 0, nil, err
	}
	for _, entry := range table {
		if entry.ID == id {
			return id, table, nil
		}
	}
	return 0, table, nil
}
