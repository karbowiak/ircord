-- +goose Up
-- Channel-wide settings shared by every IRCord member of one upstream channel.
-- network is the lower-cased host:port, channel the casefolded channel name.
CREATE TABLE IF NOT EXISTS channel_settings (
    network TEXT NOT NULL,
    channel TEXT NOT NULL,
    payload BLOB NOT NULL,
    updated_ns INTEGER NOT NULL,
    PRIMARY KEY (network, channel)
);
