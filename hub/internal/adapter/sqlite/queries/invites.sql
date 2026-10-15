-- name: CreateInvite :exec
INSERT INTO invites (code, guild_id, created_by, max_uses, uses, expires_at, revoked_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetInvite :one
SELECT * FROM invites WHERE code = ?;

-- name: UseInvite :execrows
UPDATE invites SET uses = uses + 1
WHERE code = sqlc.arg(code)
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > sqlc.arg(now))
  AND (max_uses = 0 OR uses < max_uses);

-- name: RevokeInvite :execrows
UPDATE invites SET revoked_at = ? WHERE code = ? AND revoked_at IS NULL;

-- name: ListUsableGuildInvites :many
SELECT * FROM invites
WHERE guild_id = sqlc.arg(guild_id)
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > sqlc.arg(now))
  AND (max_uses = 0 OR uses < max_uses)
ORDER BY created_at DESC, code;
