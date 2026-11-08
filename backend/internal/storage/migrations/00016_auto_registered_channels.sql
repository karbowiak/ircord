-- +goose Up
-- Channels IRCord registered with ChanServ for the account because it created them (#65),
-- so the Registration tab can say so.
CREATE TABLE IF NOT EXISTS auto_registered_channels (
    owner TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    conversation TEXT NOT NULL,
    registered_ns INTEGER NOT NULL,
    PRIMARY KEY (owner, conversation)
);
