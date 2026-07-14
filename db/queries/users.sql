-- name: CreateUser :one
INSERT INTO users (
    id, username, password_hash, display_name, email, avatar_url,
    status, is_superuser, allow_multiple_sessions, session_version,
    password_changed_at, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13
)
RETURNING id, username, password_hash, display_name, email, avatar_url,
          status, is_superuser, allow_multiple_sessions, session_version,
          password_changed_at, last_login_at, created_at, updated_at, archived_at;

-- name: GetUserByID :one
SELECT id, username, password_hash, display_name, email, avatar_url,
       status, is_superuser, allow_multiple_sessions, session_version,
       password_changed_at, last_login_at, created_at, updated_at, archived_at
FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, display_name, email, avatar_url,
       status, is_superuser, allow_multiple_sessions, session_version,
       password_changed_at, last_login_at, created_at, updated_at, archived_at
FROM users
WHERE lower(username) = lower(sqlc.arg(username));

-- name: ListUsers :many
SELECT id, username, password_hash, display_name, email, avatar_url,
       status, is_superuser, allow_multiple_sessions, session_version,
       password_changed_at, last_login_at, created_at, updated_at, archived_at
FROM users
WHERE (sqlc.arg(username_filter)::text = '' OR username ILIKE '%' || sqlc.arg(username_filter) || '%')
  AND (sqlc.arg(status_filter)::text = '' OR status = sqlc.arg(status_filter))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountUsers :one
SELECT count(*)::bigint
FROM users
WHERE (sqlc.arg(username_filter)::text = '' OR username ILIKE '%' || sqlc.arg(username_filter) || '%')
  AND (sqlc.arg(status_filter)::text = '' OR status = sqlc.arg(status_filter));

-- name: UpdateUserProfile :one
UPDATE users
SET username = $2,
    display_name = $3,
    email = $4,
    avatar_url = $5,
    updated_at = $6
WHERE id = $1
RETURNING id, username, password_hash, display_name, email, avatar_url,
          status, is_superuser, allow_multiple_sessions, session_version,
          password_changed_at, last_login_at, created_at, updated_at, archived_at;

-- name: SetUserStatus :one
UPDATE users
SET status = sqlc.arg(status)::varchar,
    archived_at = CASE WHEN sqlc.arg(status)::varchar = 'archived' THEN sqlc.arg(updated_at)::timestamptz ELSE NULL::timestamptz END,
    session_version = session_version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
RETURNING id, username, password_hash, display_name, email, avatar_url,
          status, is_superuser, allow_multiple_sessions, session_version,
          password_changed_at, last_login_at, created_at, updated_at, archived_at;

-- name: SetUserSuperuser :one
UPDATE users
SET is_superuser = $2,
    session_version = session_version + 1,
    updated_at = $3
WHERE id = $1
RETURNING id, username, password_hash, display_name, email, avatar_url,
          status, is_superuser, allow_multiple_sessions, session_version,
          password_changed_at, last_login_at, created_at, updated_at, archived_at;

-- name: SetUserMultipleSessions :one
UPDATE users
SET allow_multiple_sessions = $2,
    session_version = session_version + 1,
    updated_at = $3
WHERE id = $1
RETURNING id, username, password_hash, display_name, email, avatar_url,
          status, is_superuser, allow_multiple_sessions, session_version,
          password_changed_at, last_login_at, created_at, updated_at, archived_at;

-- name: UpdateUserPassword :one
UPDATE users
SET password_hash = $2,
    password_changed_at = $3,
    session_version = session_version + 1,
    updated_at = $3
WHERE id = $1
RETURNING id, username, password_hash, display_name, email, avatar_url,
          status, is_superuser, allow_multiple_sessions, session_version,
          password_changed_at, last_login_at, created_at, updated_at, archived_at;

-- name: UpdateUserPasswordHash :exec
UPDATE users
SET password_hash = $2,
    updated_at = $3
WHERE id = $1;

-- name: UpdateUserLastLogin :exec
UPDATE users
SET last_login_at = $2,
    updated_at = $2
WHERE id = $1;

-- name: LockSuperuserChanges :exec
SELECT pg_advisory_xact_lock(hashtext('goba.users.superusers'));

-- name: CountActiveSuperusers :one
SELECT count(*)::bigint
FROM users
WHERE is_superuser = true AND status = 'active';
