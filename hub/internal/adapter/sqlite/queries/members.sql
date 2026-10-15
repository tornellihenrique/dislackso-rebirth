-- name: AddMember :exec
INSERT INTO guild_members (guild_id, user_id, role, joined_at, updated_at) VALUES (?, ?, ?, ?, ?);

-- name: GetMember :one
SELECT * FROM guild_members WHERE guild_id = ? AND user_id = ?;

-- name: UpdateMemberRole :execrows
UPDATE guild_members SET role = ?, updated_at = ? WHERE guild_id = ? AND user_id = ?;

-- name: RemoveMember :execrows
DELETE FROM guild_members WHERE guild_id = ? AND user_id = ?;

-- name: ListGuildMembers :many
SELECT * FROM guild_members WHERE guild_id = ? ORDER BY joined_at, user_id;

-- name: ListUserMemberships :many
SELECT * FROM guild_members WHERE user_id = ? ORDER BY joined_at, guild_id;

-- name: CountGuildMembers :one
SELECT count(*) FROM guild_members WHERE guild_id = ?;
