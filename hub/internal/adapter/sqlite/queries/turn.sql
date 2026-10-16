-- name: AddTURNUsage :exec
INSERT INTO turn_usage (month, bytes) VALUES (?, ?)
ON CONFLICT (month) DO UPDATE SET bytes = bytes + excluded.bytes;

-- name: GetTURNUsage :one
SELECT bytes FROM turn_usage WHERE month = ?;
