-- +goose Up
CREATE TABLE IF NOT EXISTS account_origins (
    owner TEXT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    address TEXT NOT NULL
);

-- +goose Down
DROP TABLE account_origins;
