-- name: CreateUser :exec
INSERT INTO users (
  id, username, email, email_verified_at, password_hash, display_name, avatar_file_id,
  status_pref, custom_status, is_instance_admin, disabled_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ? AND deleted_at IS NULL;

-- name: GetUserByLogin :one
SELECT * FROM users
WHERE (username = sqlc.arg(login) OR email = sqlc.arg(login)) AND deleted_at IS NULL
LIMIT 1;

-- name: UpdateUser :execrows
UPDATE users SET
  username = ?,
  email = ?,
  email_verified_at = ?,
  password_hash = ?,
  display_name = ?,
  avatar_file_id = ?,
  status_pref = ?,
  custom_status = ?,
  is_instance_admin = ?,
  disabled_at = ?,
  updated_at = ?
WHERE id = ? AND deleted_at IS NULL;

-- name: ListUsers :many
SELECT * FROM users WHERE deleted_at IS NULL ORDER BY id;

-- name: CountUsers :one
SELECT count(*) FROM users WHERE deleted_at IS NULL;

-- name: ListGuildUsers :many
SELECT u.* FROM users u
JOIN guild_members m ON m.user_id = u.id
WHERE m.guild_id = ? AND u.deleted_at IS NULL
ORDER BY u.id;

-- Deleted accounts keep their row for history (audit log, bans) as an
-- anonymous "Deleted User"; the username and email become free again.
-- name: DeleteUser :execrows
UPDATE users SET
  username = 'deleted-' || id,
  email = 'deleted-' || id || '@deleted.invalid',
  email_verified_at = NULL,
  password_hash = '',
  display_name = '',
  avatar_file_id = NULL,
  custom_status = '',
  is_instance_admin = 0,
  updated_at = sqlc.arg(at),
  deleted_at = sqlc.arg(at)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
