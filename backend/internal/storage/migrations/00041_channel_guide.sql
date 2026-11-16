-- +goose Up
-- The server's channel guide (#258): one document, edited by server owners.
CREATE TABLE IF NOT EXISTS channel_guide (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    payload BLOB NOT NULL,
    revision INTEGER NOT NULL,
    updated_ns INTEGER NOT NULL,
    updated_by TEXT NOT NULL
);
-- Channel operators' requests to list their channel. channel is the folded channel name;
-- founder is the channel's registered founder when the request was made, so a request is
-- dropped when the channel changes hands.
CREATE TABLE IF NOT EXISTS channel_guide_requests (
    channel TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    requester TEXT NOT NULL,
    founder TEXT NOT NULL,
    group_name TEXT NOT NULL,
    description TEXT NOT NULL,
    created_ns INTEGER NOT NULL
);

-- +goose Down
DROP TABLE channel_guide_requests;
DROP TABLE channel_guide;
