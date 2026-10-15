-- name: CreateChannel :exec
INSERT INTO channels (
  id, guild_id, parent_id, type, name, topic, position, user_limit,
  drawing_allowed, max_share_quality, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetChannel :one
SELECT * FROM channels WHERE id = ?;

-- name: UpdateChannel :execrows
UPDATE channels SET
  parent_id = ?,
  name = ?,
  topic = ?,
  position = ?,
  user_limit = ?,
  drawing_allowed = ?,
  max_share_quality = ?,
  updated_at = ?
WHERE id = ?;

-- name: DeleteChannel :execrows
DELETE FROM channels WHERE id = ?;

-- name: ListGuildChannels :many
SELECT * FROM channels WHERE guild_id = ? ORDER BY position, id;

-- name: CountGuildChannels :one
SELECT count(*) FROM channels WHERE guild_id = ?;
