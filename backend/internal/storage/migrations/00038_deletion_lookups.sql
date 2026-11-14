-- +goose Up
-- Deleting a message rewrites its earlier copies in the replay log, scrubs the
-- server-window lines that carried it and updates replies that quote it. Without
-- these indexes each of those lookups reads every retained event of the account
-- or every message of the conversation, once per receiving account.
CREATE INDEX IF NOT EXISTS events_message ON events(owner, json_extract(payload, '$.data.id'))
    WHERE json_extract(payload, '$.type') IN ('message.created', 'message.updated');
-- Server-window lines are tied to a message by one of these tags.
CREATE INDEX IF NOT EXISTS events_line_msgid ON events(owner, json_extract(payload, '$.data.tags.msgid'))
    WHERE json_extract(payload, '$.type') = 'server.line' AND json_extract(payload, '$.data.tags.msgid') IS NOT NULL;
CREATE INDEX IF NOT EXISTS events_line_edit ON events(owner, json_extract(payload, '$.data.tags."+ircord/edit"'))
    WHERE json_extract(payload, '$.type') = 'server.line' AND json_extract(payload, '$.data.tags."+ircord/edit"') IS NOT NULL;
CREATE INDEX IF NOT EXISTS events_line_client ON events(owner, json_extract(payload, '$.data.tags."+ircord/client-id"'))
    WHERE json_extract(payload, '$.type') = 'server.line' AND json_extract(payload, '$.data.tags."+ircord/client-id"') IS NOT NULL;
CREATE INDEX IF NOT EXISTS events_line_label ON events(owner, json_extract(payload, '$.data.tags.label'))
    WHERE json_extract(payload, '$.type') = 'server.line' AND json_extract(payload, '$.data.tags.label') IS NOT NULL;
-- Lines inside a multiline batch carry only the batch reference.
CREATE INDEX IF NOT EXISTS events_line_batch ON events(owner, json_extract(payload, '$.data.tags.batch'))
    WHERE json_extract(payload, '$.type') = 'server.line' AND json_extract(payload, '$.data.tags.batch') IS NOT NULL;
-- BATCH lines that open or close a batch name it in their first parameter.
CREATE INDEX IF NOT EXISTS events_line_batch_ref ON events(owner, json_extract(payload, '$.data.params[0]'))
    WHERE json_extract(payload, '$.type') = 'server.line' AND json_extract(payload, '$.data.command') = 'BATCH';
CREATE INDEX IF NOT EXISTS messages_replies ON messages(owner, conversation, json_extract(payload, '$.replyTo.id'))
    WHERE json_extract(payload, '$.replyTo.id') IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS events_message;
DROP INDEX IF EXISTS events_line_msgid;
DROP INDEX IF EXISTS events_line_edit;
DROP INDEX IF EXISTS events_line_client;
DROP INDEX IF EXISTS events_line_label;
DROP INDEX IF EXISTS events_line_batch;
DROP INDEX IF EXISTS events_line_batch_ref;
DROP INDEX IF EXISTS messages_replies;
