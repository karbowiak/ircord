package activity

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"
)

// Routing checkpoints retain the focal device and deadline origin across a
// restart. Connections and leases belong to one process and never survive it.
type savedRouting struct {
	Version     int         `json:"version"`
	Serial      uint64      `json:"serial"`
	Revision    uint64      `json:"revision"`
	Generation  uint64      `json:"generation"`
	Focal       string      `json:"focal"`
	LastRenewed time.Time   `json:"lastRenewed"`
	Interval    *interval   `json:"interval,omitempty"`
	Candidates  []candidate `json:"candidates,omitempty"`
}

func (r *Router) persistLocked(owner string, a *account) {
	raw, err := r.routingDataLocked(a)
	if err != nil {
		return
	}
	r.queueLocked(owner, raw)
}
func (r *Router) routingDataLocked(a *account) ([]byte, error) {
	a.checkpointAt = r.options.Now()
	a.serial++
	return json.Marshal(savedRouting{Version: 2, Serial: a.serial, Revision: a.revision, Generation: a.generation, Focal: a.focal, LastRenewed: a.lastRenewed, Interval: a.interval, Candidates: a.activeCandidates})
}
func (r *Router) restore(owner string, raw []byte) {
	var saved savedRouting
	if json.Unmarshal(raw, &saved) != nil {
		return
	}
	if saved.Version != 2 {
		iv := decodeInterval(raw)
		if iv == nil {
			return
		}
		saved.Interval = iv
		saved.Focal = iv.Preferred
		saved.Generation = iv.Generation
	}
	a := r.account(owner)
	a.revision = saved.Revision + 1
	if a.revision < r.epochBase {
		a.revision = r.epochBase
	}
	a.serial = saved.Serial
	a.generation = saved.Generation
	a.focal = saved.Focal
	a.lastRenewed = saved.LastRenewed
	a.interval = saved.Interval
	a.activeCandidates = saved.Candidates
	if a.generation > r.generation {
		r.generation = a.generation
	}
	// Leases never survive the process. Retain the checkpoint's bounded
	// inactivity origin, rather than jumping to all-device delivery on restart.
	if a.interval == nil && a.focal != "" && !a.lastRenewed.IsZero() {
		a.interval = &interval{Generation: a.generation, T0: a.lastRenewed, Preferred: a.focal, Phase: phaseGrace, Candidates: a.activeCandidates}
		a.activeCandidates = nil
	}
	if store, ok := r.options.Store.(CandidateStore); ok && a.interval != nil && len(a.interval.Candidates) > 0 {
		a.persistenceBlocked = true
		for _, c := range a.interval.Candidates {
			if err := store.PutCandidate(owner, a.generation, c); err != nil {
				slog.Warn("Saved notification candidate could not be migrated", "error", err)
				return
			}
		}
		a.interval.Candidates = nil
		a.persistenceBlocked = false
	}
}

// queueLocked records the latest snapshot for an owner (nil removes it) and wakes the writer.
func (r *Router) queueLocked(owner string, raw []byte) {
	if r.options.Store == nil {
		return
	}
	r.persistMu.Lock()
	r.pending[owner] = raw
	r.persistMu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// flush writes every queued snapshot. Writes are serialized and each owner's latest snapshot
// replaces any older one still queued, so an older state never overwrites a newer one.
func (r *Router) flush() {
	if r.options.Store == nil {
		return
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	r.persistMu.Lock()
	batch := r.pending
	clears, prunes := r.pendingClear, r.pendingPrune
	r.pending = map[string][]byte{}
	r.pendingClear, r.pendingPrune = map[string]uint64{}, map[string]uint64{}
	r.persistMu.Unlock()
	for owner, raw := range batch {
		var err error
		if raw == nil {
			err = r.options.Store.DeleteInterval(owner)
		} else {
			err = r.options.Store.SaveInterval(owner, raw)
		}
		if err != nil {
			slog.Warn("A notification interval could not be saved", "error", err)
			r.persistMu.Lock()
			if _, newer := r.pending[owner]; !newer {
				r.pending[owner] = raw
			}
			r.persistMu.Unlock()
		}
	}
	if store, ok := r.options.Store.(CandidateStore); ok {
		for owner, gen := range clears {
			if err := store.ClearCandidates(owner, gen); err != nil {
				r.persistMu.Lock()
				if gen > r.pendingClear[owner] {
					r.pendingClear[owner] = gen
				}
				r.persistMu.Unlock()
				slog.Warn("Old notification candidates could not be removed", "error", err)
			}
		}
		if store, ok := r.options.Store.(interface{ PruneCompleted(string, uint64) error }); ok {
			for owner, gen := range prunes {
				if err := store.PruneCompleted(owner, gen); err != nil {
					r.persistMu.Lock()
					r.pendingPrune[owner] = gen
					r.persistMu.Unlock()
					slog.Warn("Completed notification candidates could not be removed", "error", err)
				}
			}
		}
	}
}

func decodeInterval(raw []byte) *interval {
	var iv interval
	if json.Unmarshal(raw, &iv) != nil || iv.T0.IsZero() || iv.Phase < phaseGrace || iv.Phase > phaseAll {
		return nil
	}
	// Older routing stored one delivered marker rather than per-device
	// receipts. Preserve the known focal receipt and skip an old all-device
	// delivery, so upgrading does not deliberately repeat either.
	var legacy struct {
		Candidates []struct {
			Delivered string `json:"delivered"`
		}
	}
	_ = json.Unmarshal(raw, &legacy)
	kept := iv.Candidates[:0:0]
	for i, c := range iv.Candidates {
		if i < len(legacy.Candidates) {
			switch legacy.Candidates[i].Delivered {
			case "all":
				continue
			case "preferred":
				c.Alerted = append(c.Alerted, iv.Preferred)
			}
		}
		kept = append(kept, c)
	}
	iv.Candidates = kept
	return &iv
}

// SQLStore keeps one small routing checkpoint per account, and pages pending
// candidates independently so an interval never truncates conversation groups.
type SQLStore struct{ DB *sql.DB }

// NewSQLStore creates the table when needed.
func NewSQLStore(db *sql.DB) (*SQLStore, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS notification_intervals(owner TEXT PRIMARY KEY, data BLOB NOT NULL)`)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS notification_candidates(owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,generation INTEGER NOT NULL,conversation TEXT NOT NULL,message TEXT NOT NULL,item BLOB NOT NULL,alerted BLOB NOT NULL DEFAULT '[]',PRIMARY KEY(owner,generation,conversation,message))`); err != nil {
		return nil, err
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS notification_routing_epoch(id INTEGER PRIMARY KEY CHECK(id=1),epoch INTEGER NOT NULL)`); err != nil {
		return nil, err
	}
	return &SQLStore{db}, nil
}

// Reserve a fresh revision range before publishing snapshots. One startup
// write makes ordering survive a crash even when newer ephemeral snapshots
// had not yet reached the coalesced checkpoint writer, or the clock moves back.
func (s *SQLStore) NextRoutingEpoch(now time.Time) (uint64, error) {
	var epoch uint64
	minimum := (uint64(now.UnixNano()) >> 32) + 1
	err := s.DB.QueryRow(`INSERT INTO notification_routing_epoch(id,epoch) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET epoch=max(epoch+1,excluded.epoch) RETURNING epoch`, minimum).Scan(&epoch)
	return epoch << 32, err
}

func (s *SQLStore) PutCandidate(owner string, generation uint64, c candidate) error {
	return putCandidate(s.DB, owner, generation, c)
}
func putCandidate(q interface {
	Exec(string, ...any) (sql.Result, error)
}, owner string, generation uint64, c candidate) error {
	raw, err := json.Marshal(c.Item)
	if err != nil {
		return err
	}
	receipts, _ := json.Marshal(c.Alerted)
	if c.Alerted == nil {
		receipts = []byte("[]")
	}
	_, err = q.Exec(`INSERT INTO notification_candidates(owner,generation,conversation,message,item,alerted) VALUES(?,?,?,?,?,?) ON CONFLICT DO NOTHING`, owner, generation, c.Conversation, c.Message, raw, receipts)
	return err
}

// The live candidate and its generation/deadline checkpoint commit together.
// An older background checkpoint cannot overwrite this transaction.
func (s *SQLStore) PutCandidateState(owner string, generation uint64, c candidate, raw []byte) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(saveRoutingSQL, owner, raw); err != nil {
		return err
	}
	var current uint64
	if err = tx.QueryRow(`SELECT json_extract(data,'$.generation') FROM notification_intervals WHERE owner=?`, owner).Scan(&current); err != nil {
		return err
	}
	if current == generation {
		if err = putCandidate(tx, owner, generation, c); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *SQLStore) CandidateGroups(owner string, generation uint64, after string, limit int) ([]string, error) {
	rows, err := s.DB.Query(`SELECT DISTINCT conversation FROM notification_candidates WHERE owner=? AND generation=? AND conversation>? ORDER BY conversation LIMIT ?`, owner, generation, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
func (s *SQLStore) CandidatePage(owner string, generation uint64, conversation, after string, limit int) ([]candidate, error) {
	rows, err := s.DB.Query(`SELECT item,alerted FROM notification_candidates WHERE owner=? AND generation=? AND conversation=? AND message>? ORDER BY message LIMIT ?`, owner, generation, conversation, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []candidate{}
	for rows.Next() {
		var raw, receipts []byte
		var c candidate
		if err = rows.Scan(&raw, &receipts); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &c.Item); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(receipts, &c.Alerted); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *SQLStore) MarkCandidate(owner string, generation uint64, conversation, message, session string) error {
	_, err := s.DB.Exec(`UPDATE notification_candidates SET alerted=json_insert(alerted,'$[#]',?) WHERE owner=? AND generation=? AND conversation=? AND message=? AND NOT EXISTS(SELECT 1 FROM json_each(notification_candidates.alerted) WHERE value=?)`, session, owner, generation, conversation, message, session)
	return err
}
func (s *SQLStore) CandidateAlerted(owner string, generation uint64, conversation, message, session string) (bool, error) {
	var seen bool
	err := s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM notification_candidates c,json_each(c.alerted) receipt WHERE c.owner=? AND c.generation=? AND c.conversation=? AND c.message=? AND receipt.value=?)`, owner, generation, conversation, message, session).Scan(&seen)
	return seen, err
}
func (s *SQLStore) ClearCandidates(owner string, generation uint64) error {
	_, err := s.DB.Exec(`DELETE FROM notification_candidates WHERE owner=? AND generation<=?`, owner, generation)
	return err
}

func (s *SQLStore) PruneCompleted(owner string, generation uint64) error {
	_, err := s.DB.Exec(`DELETE FROM notification_candidates WHERE owner=? AND generation=? AND EXISTS(SELECT 1 FROM notification_intervals r WHERE r.owner=? AND json_extract(r.data,'$.generation')=? AND json_extract(r.data,'$.interval.allDone')=1)`, owner, generation, owner, generation)
	return err
}

func (s *SQLStore) SaveInterval(owner string, data []byte) error {
	_, err := s.DB.Exec(saveRoutingSQL, owner, data)
	return err
}

const saveRoutingSQL = `INSERT INTO notification_intervals(owner,data) VALUES(?,?) ON CONFLICT(owner) DO UPDATE SET data=excluded.data WHERE coalesce(json_extract(excluded.data,'$.serial'),0)>=coalesce(json_extract(notification_intervals.data,'$.serial'),0)`

func (s *SQLStore) DeleteInterval(owner string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM notification_candidates WHERE owner=?", owner); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM notification_intervals WHERE owner=?", owner); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLStore) LoadIntervals() (map[string][]byte, error) {
	rows, err := s.DB.Query("SELECT owner,data FROM notification_intervals")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]byte{}
	for rows.Next() {
		var owner string
		var data []byte
		if err = rows.Scan(&owner, &data); err != nil {
			return nil, err
		}
		out[owner] = data
	}
	return out, rows.Err()
}
