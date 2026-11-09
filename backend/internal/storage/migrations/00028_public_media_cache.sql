-- +goose Up
-- Public resolver metadata only. No message history, accounts, extraction
-- headers, embed HTML, ingestion queues, or retained gallery references.
CREATE TABLE public_media_results (
    namespace TEXT NOT NULL,
    cache_key TEXT NOT NULL,
    payload BLOB NOT NULL CHECK(length(payload)<=524288),
    expires_ns INTEGER NOT NULL,
    PRIMARY KEY(namespace,cache_key)
);
CREATE INDEX public_media_results_expiry ON public_media_results(namespace,expires_ns);

-- Every upload is reserved before bytes are written. Cleanup retries do not
-- extend its absolute expiry. Keys contain random bytes, never source URLs.
CREATE TABLE public_media_objects (
    namespace TEXT NOT NULL,
    object_key TEXT NOT NULL,
    expires_ns INTEGER NOT NULL,
    retry_ns INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(namespace,object_key)
);
CREATE INDEX public_media_objects_expiry ON public_media_objects(namespace,expires_ns,retry_ns);

-- No down migration: removing object records would abandon retained bytes.
-- Drain the cache prefix before removing these tables.
