-- name: CreateGuild :exec
INSERT INTO guilds (id, name, icon_file_id, owner_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetGuild :one
SELECT * FROM guilds WHERE id = ?;

-- name: UpdateGuild :execrows
UPDATE guilds SET name = ?, icon_file_id = ?, owner_id = ?, updated_at = ? WHERE id = ?;

-- name: DeleteGuild :execrows
DELETE FROM guilds WHERE id = ?;

-- name: CountGuildsOwnedBy :one
SELECT count(*) FROM guilds WHERE owner_id = ?;

-- name: ListUserGuilds :many
SELECT g.* FROM guilds g
JOIN guild_members m ON m.guild_id = g.id
WHERE m.user_id = ?
ORDER BY m.joined_at, g.id;
