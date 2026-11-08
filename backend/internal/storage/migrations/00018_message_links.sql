-- +goose Up
-- The message link index: one row per catalogued item of an account's stored
-- message, in message order (idx). It is derived from messages and rewritten
-- whenever the message is stored again, so edits, deletions and preview results
-- replace a message's rows instead of adding more. kind says what the item is
-- (image, gif; a link catalog adds its own kinds to the same table, with its
-- own partial index). Only the original URL, a smaller rendition the source
-- names (thumb_url) and an upload's file ID are stored, never the media itself.
-- The network is not stored: it is the conversation's, read where needed.
CREATE TABLE IF NOT EXISTS message_links (
    owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    idx INTEGER NOT NULL,
    conversation TEXT NOT NULL,
    sender TEXT NOT NULL,
    created_ns INTEGER NOT NULL,
    kind TEXT NOT NULL,
    source TEXT NOT NULL,
    url TEXT NOT NULL,
    thumb_url TEXT NOT NULL DEFAULT '',
    file_id TEXT NOT NULL DEFAULT '',
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (owner, message, idx)
);
-- Gallery pages read images newest first without sorting; the predicate must
-- match galleryKinds in storage/links.go for SQLite to use this index.
CREATE INDEX IF NOT EXISTS message_links_gallery ON message_links(owner, conversation, created_ns DESC, message DESC, idx) WHERE kind IN ('image','gif');
-- Checkpoints of background jobs over stored history, such as indexing the
-- messages stored before the index existed. last_rowid is the messages rowid
-- the job has finished through; version is the rules it ran under.
CREATE TABLE IF NOT EXISTS index_jobs (
    name TEXT PRIMARY KEY,
    version INTEGER NOT NULL,
    last_rowid INTEGER NOT NULL DEFAULT 0,
    done INTEGER NOT NULL DEFAULT 0,
    updated_ns INTEGER NOT NULL
);
