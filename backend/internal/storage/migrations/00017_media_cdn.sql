-- +goose Up
CREATE TABLE IF NOT EXISTS media_objects (
 hash TEXT PRIMARY KEY,
 size INTEGER NOT NULL,
 content_type TEXT NOT NULL,
 width INTEGER NOT NULL DEFAULT 0,
 height INTEGER NOT NULL DEFAULT 0,
 animated INTEGER NOT NULL DEFAULT 0,
 created_ns INTEGER NOT NULL,
 removed INTEGER NOT NULL DEFAULT 0,
 purged INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS media_sources (
 source_hash TEXT PRIMARY KEY,
 object_hash TEXT NOT NULL REFERENCES media_objects(hash),
 removed INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS media_sources_object ON media_sources(object_hash);
CREATE TABLE IF NOT EXISTS media_references (
 owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
 conversation TEXT NOT NULL,
 message TEXT NOT NULL,
 source_hash TEXT NOT NULL,
 PRIMARY KEY(owner,conversation,message,source_hash)
);
CREATE INDEX IF NOT EXISTS media_references_source ON media_references(source_hash);

CREATE TABLE IF NOT EXISTS media_denied_sources (source_hash TEXT PRIMARY KEY);

-- +goose Down
DROP TABLE media_denied_sources;
DROP TABLE media_references;
DROP TABLE media_sources;
DROP TABLE media_objects;
