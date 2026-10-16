-- +goose Up
-- Bytes voice clients reported through a relay (TURN), per calendar month,
-- for the budget guard.
CREATE TABLE turn_usage (
    month TEXT PRIMARY KEY,
    bytes INTEGER NOT NULL
) STRICT;

-- +goose Down
DROP TABLE turn_usage;
