-- name: CreateEmailToken :exec
INSERT INTO email_tokens (
  id, user_id, purpose, token_hash, payload, expires_at, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetEmailTokenByHash :one
SELECT * FROM email_tokens WHERE token_hash = ?;

-- name: UseEmailToken :execrows
UPDATE email_tokens SET used_at = ? WHERE id = ? AND used_at IS NULL;

-- name: ExpireUserTokens :exec
UPDATE email_tokens SET used_at = sqlc.arg(used_at)
WHERE user_id = sqlc.arg(user_id) AND purpose = sqlc.arg(purpose) AND used_at IS NULL;

-- name: ListEmailTokensByPurpose :many
SELECT * FROM email_tokens WHERE purpose = ? ORDER BY id DESC;

-- name: DeleteEmailToken :execrows
DELETE FROM email_tokens WHERE id = ? AND purpose = ?;
