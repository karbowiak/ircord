-- +goose Up
-- IRCord server owners (#165): administration authority, bound to immutable account IDs so a
-- later account that reuses a name never inherits it. source is 'environment' (IRCORD_OWNERS),
-- 'operator' (the bearer-token admin API) or 'owner' (granted from the Administration panel).
CREATE TABLE IF NOT EXISTS server_owners (
    account_id TEXT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    granted_by TEXT NOT NULL DEFAULT '',
    granted_ns INTEGER NOT NULL
);
-- A configured owner username resolves once, to an account that already existed when the name
-- was first configured, and stays pinned to that account. Registering a configured name later
-- never makes anyone an owner.
CREATE TABLE IF NOT EXISTS owner_bootstrap (
    username TEXT PRIMARY KEY COLLATE NOCASE,
    first_seen_ns INTEGER NOT NULL,
    account_id TEXT
);
-- What owners did in the Administration panel, and to which target. No credential material.
CREATE TABLE IF NOT EXISTS owner_audit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    target TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    outcome TEXT NOT NULL,
    created_ns INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS owner_audit_created ON owner_audit(created_ns DESC);
