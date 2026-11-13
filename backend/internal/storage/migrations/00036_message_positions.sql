-- +goose Up
-- Each new message takes the next position in its conversation. Without this
-- index, finding that position walks the conversation's whole history.
CREATE INDEX IF NOT EXISTS messages_next_position ON messages(owner,conversation,position);
