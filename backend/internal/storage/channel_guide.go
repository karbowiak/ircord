package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// ErrStaleRevision refuses a channel guide save based on an older revision.
var ErrStaleRevision = errors.New("the channel guide changed since it was loaded")

// StoredChannelGuide is the saved channel guide document. Revision 0 means no guide was saved.
type StoredChannelGuide struct {
	Payload   json.RawMessage
	Revision  int64
	UpdatedAt time.Time
	UpdatedBy string
}

// ChannelGuide reads the saved guide; a server without one returns revision 0.
func (s *Store) ChannelGuide(ctx context.Context) (StoredChannelGuide, error) {
	var v StoredChannelGuide
	var raw []byte
	var ns int64
	err := s.DB.QueryRowContext(ctx, "SELECT payload, revision, updated_ns, updated_by FROM channel_guide WHERE id=1").Scan(&raw, &v.Revision, &ns, &v.UpdatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return StoredChannelGuide{}, nil
	}
	if err != nil {
		return v, err
	}
	v.Payload, v.UpdatedAt = raw, time.Unix(0, ns).UTC()
	return v, nil
}

// SaveChannelGuide replaces the guide when revision is still the saved one, and removes the
// listing requests for the channels it now lists. It returns the new revision.
func (s *Store) SaveChannelGuide(ctx context.Context, payload json.RawMessage, revision int64, by string, listed []string) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var current int64
	if err = tx.QueryRowContext(ctx, "SELECT revision FROM channel_guide WHERE id=1").Scan(&current); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if current != revision {
		return 0, ErrStaleRevision
	}
	next := current + 1
	if _, err = tx.ExecContext(ctx, "INSERT INTO channel_guide(id, payload, revision, updated_ns, updated_by) VALUES(1,?,?,?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload, revision=excluded.revision, updated_ns=excluded.updated_ns, updated_by=excluded.updated_by", []byte(payload), next, time.Now().UnixNano(), by); err != nil {
		return 0, err
	}
	for _, channel := range listed {
		if _, err = tx.ExecContext(ctx, "DELETE FROM channel_guide_requests WHERE channel=?", channel); err != nil {
			return 0, err
		}
	}
	return next, tx.Commit()
}

// ChannelGuideRequest is a channel operator's request to list their channel in the guide.
type ChannelGuideRequest struct {
	Channel     string
	Name        string
	AccountID   string
	Requester   string
	Founder     string
	Group       string
	Description string
	CreatedAt   time.Time
}

// ChannelGuideRequests returns every pending request, oldest first.
func (s *Store) ChannelGuideRequests(ctx context.Context) ([]ChannelGuideRequest, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT channel, name, account_id, requester, founder, group_name, description, created_ns FROM channel_guide_requests ORDER BY created_ns, channel")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChannelGuideRequest{}
	for rows.Next() {
		var v ChannelGuideRequest
		var ns int64
		if err = rows.Scan(&v.Channel, &v.Name, &v.AccountID, &v.Requester, &v.Founder, &v.Group, &v.Description, &ns); err != nil {
			return nil, err
		}
		v.CreatedAt = time.Unix(0, ns).UTC()
		out = append(out, v)
	}
	return out, rows.Err()
}

// ChannelGuideRequest returns the pending request for one folded channel name.
func (s *Store) ChannelGuideRequest(ctx context.Context, channel string) (ChannelGuideRequest, error) {
	var v ChannelGuideRequest
	var ns int64
	err := s.DB.QueryRowContext(ctx, "SELECT channel, name, account_id, requester, founder, group_name, description, created_ns FROM channel_guide_requests WHERE channel=?", channel).Scan(&v.Channel, &v.Name, &v.AccountID, &v.Requester, &v.Founder, &v.Group, &v.Description, &ns)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	v.CreatedAt = time.Unix(0, ns).UTC()
	return v, err
}

// SaveChannelGuideRequest creates or replaces the request for a channel. A replacement keeps
// its place in the queue. It refuses a new request once max requests are pending.
func (s *Store) SaveChannelGuideRequest(ctx context.Context, v ChannelGuideRequest, max int) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists bool
	var pending int
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM channel_guide_requests WHERE channel=?), (SELECT count(*) FROM channel_guide_requests)", v.Channel).Scan(&exists, &pending); err != nil {
		return err
	}
	if !exists && pending >= max {
		return ErrRequestQueueFull
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO channel_guide_requests(channel, name, account_id, requester, founder, group_name, description, created_ns) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(channel) DO UPDATE SET name=excluded.name, account_id=excluded.account_id, requester=excluded.requester, founder=excluded.founder, group_name=excluded.group_name, description=excluded.description", v.Channel, v.Name, v.AccountID, v.Requester, v.Founder, v.Group, v.Description, time.Now().UnixNano()); err != nil {
		return err
	}
	return tx.Commit()
}

// ErrRequestQueueFull refuses a new listing request while too many are pending.
var ErrRequestQueueFull = errors.New("too many channel guide requests are waiting")

// DeleteChannelGuideRequest removes the request for one folded channel name. It reports
// whether there was one.
func (s *Store) DeleteChannelGuideRequest(ctx context.Context, channel string) (bool, error) {
	result, err := s.DB.ExecContext(ctx, "DELETE FROM channel_guide_requests WHERE channel=?", channel)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}
