-- +goose Up
-- The private attention inbox: one row per account and canonical message that
-- mentioned the account, replied to it, or arrived in a direct conversation.
-- reasons is a bit set (1 mention, 2 reply, 4 direct). dismissed_ns is inbox
-- state only and never a conversation read marker.
CREATE TABLE IF NOT EXISTS inbox_items (
    owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    conversation TEXT NOT NULL,
    network TEXT NOT NULL,
    sender TEXT NOT NULL,
    reasons INTEGER NOT NULL,
    created_ns INTEGER NOT NULL,
    dismissed_ns INTEGER,
    PRIMARY KEY (owner, message)
);
CREATE INDEX IF NOT EXISTS inbox_items_page ON inbox_items(owner, created_ns DESC, message DESC);
