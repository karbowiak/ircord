package storage

import (
	"context"
	"database/sql"
)

// Development builds applied the first version of migration 29 before device
// preference revisions were added. Reconcile that schema forwards, preserving
// every desired setting and recorded capability. Fresh databases already have
// the column and take the same migration path without altering it.
func reconcileDeviceNotificationSettings(ctx context.Context, tx *sql.Tx) error {
	found, err := hasColumn(ctx, tx, "session_notification_settings", "revision")
	if err != nil {
		return err
	}
	if !found {
		if _, err = tx.ExecContext(ctx, `ALTER TABLE session_notification_settings ADD COLUMN revision INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	// An already chosen Off must not become an untouched setup default. Earlier
	// records that predate explicit-choice metadata retain that conservative
	// interpretation; no delivery mode or browser permission is changed.
	_, err = tx.ExecContext(ctx, `UPDATE session_notification_settings SET preferences=json_set(preferences,'$.explicit',json('true')) WHERE json_extract(preferences,'$.initialized')=1 AND json_type(preferences,'$.explicit') IS NULL`)
	return err
}
