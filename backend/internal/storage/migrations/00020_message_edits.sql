-- +goose Up
-- Message edits (#5) are stored on the original message's payload. These partial
-- indexes cover only edited messages and edits still waiting for their original,
-- so looking either up never scans a conversation's whole history.
CREATE INDEX IF NOT EXISTS messages_edited ON messages(owner, conversation) WHERE json_extract(payload, '$.editedAt') IS NOT NULL;
CREATE INDEX IF NOT EXISTS messages_edit_of ON messages(owner, conversation, json_extract(payload, '$.editOf')) WHERE json_extract(payload, '$.editOf') IS NOT NULL;
