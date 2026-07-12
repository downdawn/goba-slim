-- name: GetSchemaVersion :one
SELECT version, name, applied_at
FROM schema_migrations
ORDER BY version DESC
LIMIT 1;

-- name: CountPublicTables :one
SELECT count(*)::bigint
FROM pg_catalog.pg_tables
WHERE schemaname = 'public';
