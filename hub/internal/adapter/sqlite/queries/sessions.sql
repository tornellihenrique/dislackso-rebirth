-- name: CreateSession :exec
INSERT INTO sessions (
  id, user_id, token_hash, created_at, last_seen_at, expires_at, user_agent, ip
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE token_hash = ?;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id = ?;

-- name: RevokeSession :execrows
UPDATE sessions SET revoked_at = sqlc.arg(revoked_at)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND revoked_at IS NULL;

-- name: RevokeUserSessions :execrows
UPDATE sessions SET revoked_at = sqlc.arg(revoked_at)
WHERE user_id = sqlc.arg(user_id) AND id != sqlc.arg(keep_id) AND revoked_at IS NULL;

-- name: ListActiveSessions :many
SELECT * FROM sessions
WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL AND expires_at > sqlc.arg(now)
ORDER BY last_seen_at DESC;
