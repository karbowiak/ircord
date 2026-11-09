-- +goose Up
CREATE INDEX IF NOT EXISTS messages_offline_page ON messages(owner,created_ns DESC,id DESC);

-- +goose Down
DROP INDEX IF EXISTS messages_offline_page;
