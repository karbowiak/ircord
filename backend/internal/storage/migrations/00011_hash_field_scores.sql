-- +goose Up
-- The instance-wide Hash Field high score table (Settings → About). Only the top entries are
-- kept; the table starts with two entries.
CREATE TABLE IF NOT EXISTS hash_field_scores (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    score INTEGER NOT NULL,
    created_ns INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS hash_field_scores_rank ON hash_field_scores(score DESC, id ASC);
INSERT INTO hash_field_scores(name, score, created_ns) VALUES
    ('Michael Karbowiak', 97909, 0),
    ('Peter Petermann', 22831, 0);
