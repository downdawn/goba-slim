-- name: CreateSystemConfig :one
INSERT INTO system_configs (key, value, value_type, is_public, description, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING key, value, value_type, is_public, description, created_at, updated_at;

-- name: GetSystemConfig :one
SELECT key, value, value_type, is_public, description, created_at, updated_at
FROM system_configs
WHERE key = $1;

-- name: ListSystemConfigs :many
SELECT key, value, value_type, is_public, description, created_at, updated_at
FROM system_configs
ORDER BY key
LIMIT 1000;

-- name: ListPublicSystemConfigs :many
SELECT key, value, value_type, is_public, description, created_at, updated_at
FROM system_configs
WHERE is_public = true
ORDER BY key
LIMIT 1000;

-- name: UpdateSystemConfig :one
UPDATE system_configs
SET value = $2,
    value_type = $3,
    is_public = $4,
    description = $5,
    updated_at = $6
WHERE key = $1
RETURNING key, value, value_type, is_public, description, created_at, updated_at;

-- name: DeleteSystemConfig :execrows
DELETE FROM system_configs
WHERE key = $1;
