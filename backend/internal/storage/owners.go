package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Owner sources, as stored in server_owners.source.
const (
	OwnerFromEnvironment = "environment"
	OwnerFromOperator    = "operator"
	OwnerFromOwner       = "owner"
)

// ErrLastOwner refuses to remove the only remaining server owner.
var ErrLastOwner = errors.New("the server needs at least one owner")

// ErrOwnerFromEnvironment refuses to revoke an owner configured in IRCORD_OWNERS from the panel.
var ErrOwnerFromEnvironment = errors.New("this owner is configured in the server environment")

// IsOwner reports whether a live account is a server owner. It is read on every privileged
// request, so a revocation takes effect immediately.
func (s *Store) IsOwner(accountID string) (bool, error) {
	var owner bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM server_owners o JOIN accounts a ON a.id=o.account_id WHERE o.account_id=? AND a.deleted=0)", accountID).Scan(&owner)
	return owner, err
}

// GrantOwner makes a live account a server owner. Granting an existing owner keeps its source.
func (s *Store) GrantOwner(accountID, source, grantedBy string) error {
	result, err := s.DB.Exec("INSERT INTO server_owners(account_id,source,granted_by,granted_ns) SELECT id,?,?,? FROM accounts WHERE id=? AND deleted=0 AND username IS NOT NULL ON CONFLICT(account_id) DO NOTHING", source, grantedBy, time.Now().UnixNano(), accountID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		if owner, err := s.IsOwner(accountID); err != nil || owner {
			return err
		}
		return ErrNotFound
	}
	return nil
}

// RevokeOwner removes a server owner. It refuses the last owner and owners configured in the
// environment, unless fromEnvironment (startup applying a changed IRCORD_OWNERS).
func (s *Store) RevokeOwner(accountID string, fromEnvironment bool) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var source string
	if err = tx.QueryRow("SELECT source FROM server_owners WHERE account_id=?", accountID).Scan(&source); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if source == OwnerFromEnvironment && !fromEnvironment {
		return ErrOwnerFromEnvironment
	}
	var others int
	if err = tx.QueryRow("SELECT count(*) FROM server_owners o JOIN accounts a ON a.id=o.account_id WHERE o.account_id<>? AND a.deleted=0", accountID).Scan(&others); err != nil {
		return err
	}
	if others == 0 && !fromEnvironment {
		return ErrLastOwner
	}
	if _, err = tx.Exec("DELETE FROM server_owners WHERE account_id=?", accountID); err != nil {
		return err
	}
	return tx.Commit()
}

// ApplyConfiguredOwners makes the accounts named in IRCORD_OWNERS owners, and removes
// environment-sourced ownership from names no longer configured. A name is pinned to the
// account that held it, already existing, when the name was first configured; an account
// created later under a configured name is never granted. It returns the configured names that
// don't resolve.
func (s *Store) ApplyConfiguredOwners(usernames []string) ([]string, error) {
	now := time.Now().UnixNano()
	unresolved := []string{}
	keep := map[string]bool{}
	for _, name := range usernames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, err := s.DB.Exec("INSERT INTO owner_bootstrap(username,first_seen_ns) VALUES(?,?) ON CONFLICT(username) DO NOTHING", name, now); err != nil {
			return nil, err
		}
		if _, err := s.DB.Exec(`UPDATE owner_bootstrap SET account_id=(
			SELECT a.id FROM accounts a WHERE a.username=owner_bootstrap.username COLLATE NOCASE AND a.deleted=0 AND a.created_ns<owner_bootstrap.first_seen_ns)
			WHERE username=? AND account_id IS NULL`, name); err != nil {
			return nil, err
		}
		var id sql.NullString
		if err := s.DB.QueryRow("SELECT b.account_id FROM owner_bootstrap b JOIN accounts a ON a.id=b.account_id AND a.deleted=0 WHERE b.username=?", name).Scan(&id); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if !id.Valid {
			unresolved = append(unresolved, name)
			continue
		}
		keep[id.String] = true
		// An owner granted another way becomes environment-managed while configured.
		if _, err := s.DB.Exec("INSERT INTO server_owners(account_id,source,granted_by,granted_ns) VALUES(?,?,'',?) ON CONFLICT(account_id) DO UPDATE SET source=excluded.source", id.String, OwnerFromEnvironment, now); err != nil {
			return nil, err
		}
	}
	rows, err := s.DB.Query("SELECT account_id FROM server_owners WHERE source=?", OwnerFromEnvironment)
	if err != nil {
		return nil, err
	}
	stale := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		if !keep[id] {
			stale = append(stale, id)
		}
	}
	rows.Close()
	for _, id := range stale {
		if err = s.RevokeOwner(id, true); err != nil {
			return nil, err
		}
	}
	return unresolved, nil
}

// AdminUser is one IRCord account as the Administration panel lists it. It holds no
// credentials, sessions or private content.
type AdminUser struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	CreatedAt   time.Time `json:"createdAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
	Owner       bool      `json:"owner"`
	OwnerSource string    `json:"ownerSource,omitempty" enum:"environment,operator,owner"`
	Sessions    int       `json:"sessions" minimum:"0"`
	Networks    int       `json:"networks" minimum:"0"`
}

const adminUserColumns = `a.id, a.username, a.created_ns, a.last_seen_ns, COALESCE(o.source,''),
	(SELECT count(*) FROM sessions s WHERE s.owner=a.id AND s.revoked=0 AND s.expires_ns>?),
	(SELECT count(*) FROM networks n WHERE n.owner=a.id)`

func scanAdminUser(row interface{ Scan(...any) error }) (AdminUser, error) {
	var u AdminUser
	var created, seen int64
	if err := row.Scan(&u.ID, &u.Username, &created, &seen, &u.OwnerSource, &u.Sessions, &u.Networks); err != nil {
		return u, err
	}
	u.CreatedAt, u.LastSeenAt, u.Owner = time.Unix(0, created).UTC(), time.Unix(0, seen).UTC(), u.OwnerSource != ""
	return u, nil
}

// AdminUsers lists live accounts whose username contains query, owners first, then by name. The
// page is bounded; offset continues it.
func (s *Store) AdminUsers(ctx context.Context, query string, offset, limit int) ([]AdminUser, bool, error) {
	pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(strings.ToLower(query)) + "%"
	rows, err := s.DB.QueryContext(ctx, "SELECT "+adminUserColumns+` FROM accounts a LEFT JOIN server_owners o ON o.account_id=a.id
		WHERE a.deleted=0 AND a.username IS NOT NULL AND lower(a.username) LIKE ? ESCAPE '\'
		ORDER BY o.account_id IS NULL, lower(a.username) LIMIT ? OFFSET ?`, time.Now().UnixNano(), pattern, limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []AdminUser{}
	for rows.Next() {
		u, err := scanAdminUser(rows)
		if err != nil {
			return nil, false, err
		}
		out = append(out, u)
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return out, more, rows.Err()
}

// AdminUser returns one live account by ID or username.
func (s *Store) AdminUser(ctx context.Context, idOrUsername string) (AdminUser, error) {
	u, err := scanAdminUser(s.DB.QueryRowContext(ctx, "SELECT "+adminUserColumns+` FROM accounts a LEFT JOIN server_owners o ON o.account_id=a.id
		WHERE a.deleted=0 AND a.username IS NOT NULL AND (a.id=? OR a.username=? COLLATE NOCASE)`, time.Now().UnixNano(), idOrUsername, idOrUsername))
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return u, err
}

// OwnerEvent is one audited Administration panel action.
type OwnerEvent struct {
	Actor   string    `json:"actor"`
	Action  string    `json:"action"`
	Target  string    `json:"target"`
	Reason  string    `json:"reason,omitempty"`
	Outcome string    `json:"outcome"`
	At      time.Time `json:"at"`
}

// RecordOwnerEvent appends an owner action to the audit log. actor and target are usernames,
// readable after the accounts are gone.
func (s *Store) RecordOwnerEvent(e OwnerEvent) error {
	clip := func(v string, n int) string {
		if r := []rune(v); len(r) > n {
			return string(r[:n])
		}
		return v
	}
	_, err := s.DB.Exec("INSERT INTO owner_audit(actor,action,target,reason,outcome,created_ns) VALUES(?,?,?,?,?,?)", clip(e.Actor, 100), clip(e.Action, 64), clip(e.Target, 200), clip(e.Reason, 500), clip(e.Outcome, 64), time.Now().UnixNano())
	return err
}

// OwnerEvents returns the owner audit log, newest first.
func (s *Store) OwnerEvents(ctx context.Context, limit int) ([]OwnerEvent, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT actor,action,target,reason,outcome,created_ns FROM owner_audit ORDER BY created_ns DESC, id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OwnerEvent{}
	for rows.Next() {
		var e OwnerEvent
		var at int64
		if err = rows.Scan(&e.Actor, &e.Action, &e.Target, &e.Reason, &e.Outcome, &at); err != nil {
			return nil, err
		}
		e.At = time.Unix(0, at).UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}
