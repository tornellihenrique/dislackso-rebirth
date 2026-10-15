-- +goose Up
-- Server mute and deafen outlive voice connections, so leaving and joining
-- again does not lift them.
ALTER TABLE guild_members ADD COLUMN server_mute INTEGER NOT NULL DEFAULT 0;
ALTER TABLE guild_members ADD COLUMN server_deaf INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE guild_members DROP COLUMN server_deaf;
ALTER TABLE guild_members DROP COLUMN server_mute;
