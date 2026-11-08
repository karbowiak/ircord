-- +goose Up
CREATE TABLE IF NOT EXISTS credential_operations (
    username TEXT PRIMARY KEY COLLATE NOCASE,
    payload BLOB NOT NULL
);
