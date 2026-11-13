-- +goose Up
-- Uninitialized devices stay off until their current client imports its local
-- preference. A remote edit initializes the record and wins over a later import.
CREATE TABLE IF NOT EXISTS session_notification_settings (
    session TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 0,
    preferences BLOB NOT NULL,
    permission TEXT NOT NULL DEFAULT 'unknown',
    capability_ns INTEGER NOT NULL DEFAULT 0,
    last_active_ns INTEGER NOT NULL DEFAULT 0,
    form_factor TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS session_notification_settings_owner ON session_notification_settings(owner);
