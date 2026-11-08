-- +goose Up
-- Retire derived preview/gallery data while preserving messages and uploads.
DROP TABLE IF EXISTS media_references;
DROP TABLE IF EXISTS media_denied_sources;
DROP TABLE IF EXISTS media_sources;
DROP TABLE IF EXISTS media_objects;
-- The upstream Links catalog still uses these derived tables. Rebuild its
-- text-only index without retaining gallery entries or preview state.
DELETE FROM message_links;
DELETE FROM index_jobs WHERE name='message-links';
DROP INDEX IF EXISTS message_links_gallery;

UPDATE messages SET payload = json_remove(payload, '$.preview', '$.previewSource', '$.embeds', '$.media', '$.attachment.hosted')
WHERE json_valid(payload) AND (
    json_type(payload, '$.preview') IS NOT NULL OR json_type(payload, '$.previewSource') IS NOT NULL OR
    json_type(payload, '$.embeds') IS NOT NULL OR json_type(payload, '$.media') IS NOT NULL OR
    json_type(payload, '$.attachment.hosted') IS NOT NULL
);

-- Durable replay uses raw JSON: remove retired message fields there too.
UPDATE events SET payload = json_remove(payload, '$.data.preview', '$.data.previewSource', '$.data.embeds', '$.data.media', '$.data.attachment.hosted')
WHERE json_valid(payload) AND json_extract(payload, '$.type') IN ('message.created', 'message.updated');

UPDATE entities SET payload = json_remove(payload, '$.previewMode', '$.channelSettings.previewMode', '$.channelSettings.trustedMediaHosts')
WHERE kind='conversation' AND json_valid(payload);
UPDATE channel_settings SET payload = json_remove(payload, '$.previewMode', '$.trustedMediaHosts')
WHERE json_valid(payload);
UPDATE events SET payload = json_remove(payload, '$.data.previewMode', '$.data.channelSettings.previewMode', '$.data.channelSettings.trustedMediaHosts')
WHERE json_valid(payload) AND json_extract(payload, '$.type')='conversation.updated';
