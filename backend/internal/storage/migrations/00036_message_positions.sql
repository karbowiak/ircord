-- +goose Up
-- Each new message takes the next position in its conversation. Without this
-- index, finding that position walks the conversation's whole history.
CREATE INDEX IF NOT EXISTS messages_next_position ON messages(owner,conversation,position);
-- Party messages are looked up by upstream ID as they arrive.
CREATE INDEX IF NOT EXISTS party_history_upstream ON party_history(party,json_extract(payload,'$.upstreamId'));
