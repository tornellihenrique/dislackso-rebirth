-- name: CreateFile :exec
INSERT INTO files (
  id, owner_id, purpose, sha256, size, mime, original_name, width, height, storage_key, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetFile :one
SELECT * FROM files WHERE id = ?;
