-- +goose Up
-- Operator actions taken through the administration API: what was done to which target, when,
-- and how it ended. Rows never contain credential material.
CREATE TABLE IF NOT EXISTS admin_audit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action TEXT NOT NULL,
    target TEXT NOT NULL,
    outcome TEXT NOT NULL,
    created_ns INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS admin_audit_created ON admin_audit(created_ns DESC);
