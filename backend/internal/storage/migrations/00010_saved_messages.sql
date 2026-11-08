-- +goose Up
-- Private bookmarks: one row per account and canonical stored message.
CREATE TABLE IF NOT EXISTS saved_messages (
    owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    conversation TEXT NOT NULL,
    network TEXT NOT NULL,
    saved_ns INTEGER NOT NULL,
    PRIMARY KEY (owner, message)
);
CREATE INDEX IF NOT EXISTS saved_messages_page ON saved_messages(owner, saved_ns DESC, message DESC);
