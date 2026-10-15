-- name: ListSettings :many
SELECT * FROM instance_settings;

-- name: UpsertSetting :exec
INSERT INTO instance_settings (key, value_json, updated_at) VALUES (?, ?, ?)
ON CONFLICT (key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at;
