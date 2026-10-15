-- name: CreateBan :exec
INSERT INTO guild_bans (guild_id, user_id, banned_by, reason, created_at) VALUES (?, ?, ?, ?, ?);

-- name: GetBan :one
SELECT * FROM guild_bans WHERE guild_id = ? AND user_id = ?;

-- name: DeleteBan :execrows
DELETE FROM guild_bans WHERE guild_id = ? AND user_id = ?;

-- name: ListGuildBans :many
SELECT * FROM guild_bans WHERE guild_id = ? ORDER BY created_at DESC, user_id;
