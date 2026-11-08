-- +goose Up
-- Usernames of deleted accounts. They are never reused: the associated IRC server keeps
-- channel access entries under the account name, so a new owner would inherit them.
-- unregister_pending marks names whose account on the associated server still has to be
-- unregistered.
CREATE TABLE IF NOT EXISTS retired_usernames (
    username TEXT PRIMARY KEY COLLATE NOCASE,
    retired_ns INTEGER NOT NULL,
    unregister_pending INTEGER NOT NULL DEFAULT 0
);
