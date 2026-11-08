-- +goose Up
-- Earlier development media migrations used version 20 before message editing
-- landed upstream. Restore its indexes even when that version is already logged.
CREATE INDEX IF NOT EXISTS messages_edited ON messages(owner, conversation) WHERE json_extract(payload, '$.editedAt') IS NOT NULL;
CREATE INDEX IF NOT EXISTS messages_edit_of ON messages(owner, conversation, json_extract(payload, '$.editOf')) WHERE json_extract(payload, '$.editOf') IS NOT NULL;
