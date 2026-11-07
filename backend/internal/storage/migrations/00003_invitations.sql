-- +goose Up
CREATE TABLE IF NOT EXISTS invitations (
  owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  network TEXT NOT NULL,
  channel_key TEXT NOT NULL,
  id TEXT NOT NULL,
  channel TEXT NOT NULL,
  inviter_id TEXT NOT NULL,
  inviter_nick TEXT NOT NULL,
  created_ns INTEGER NOT NULL,
  rejected INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(owner,network,channel_key),
  UNIQUE(owner,id)
);
CREATE INDEX IF NOT EXISTS invitations_pending ON invitations(owner,rejected,created_ns DESC);
