-- name: AppendAudit :exec
INSERT INTO audit_log (id, guild_id, actor_id, action, target_type, target_id, data_json, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAudit :many
SELECT * FROM audit_log
WHERE guild_id = sqlc.arg(guild_id) AND id < sqlc.arg(before)
ORDER BY id DESC
LIMIT sqlc.arg(page_size);
