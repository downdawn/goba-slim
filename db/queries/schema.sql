-- name: GetSchemaVersion :one
SELECT version, name, applied_at
FROM schema_migrations
ORDER BY version DESC
LIMIT 1;

-- name: ListPublicTables :many
SELECT tablename
FROM pg_catalog.pg_tables
WHERE schemaname = 'public'
ORDER BY tablename;
