-- +goose Up
CREATE TABLE IF NOT EXISTS party_services_identities (
 id TEXT PRIMARY KEY,
 account TEXT NOT NULL COLLATE NOCASE,
 registered_ns INTEGER NOT NULL,
 payload BLOB NOT NULL,
 UNIQUE(account,registered_ns)
);
CREATE TABLE IF NOT EXISTS party_local_bindings (
 owner TEXT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
 principal TEXT NOT NULL REFERENCES party_services_identities(id)
);
