CREATE TABLE users (
    id uuid PRIMARY KEY,
    username varchar(64) NOT NULL,
    password_hash text NOT NULL,
    display_name varchar(64) NOT NULL,
    email varchar(254),
    avatar_url text,
    status varchar(16) NOT NULL DEFAULT 'active',
    is_superuser boolean NOT NULL DEFAULT false,
    allow_multiple_sessions boolean NOT NULL DEFAULT false,
    session_version bigint NOT NULL DEFAULT 1,
    password_changed_at timestamptz NOT NULL,
    last_login_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    archived_at timestamptz,
    CONSTRAINT users_username_format CHECK (username ~ '^[A-Za-z0-9][A-Za-z0-9._-]{2,63}$'),
    CONSTRAINT users_display_name_length CHECK (char_length(display_name) BETWEEN 1 AND 64),
    CONSTRAINT users_status CHECK (status IN ('active', 'disabled', 'archived')),
    CONSTRAINT users_session_version CHECK (session_version > 0),
    CONSTRAINT users_archived_state CHECK ((status = 'archived') = (archived_at IS NOT NULL))
);

CREATE UNIQUE INDEX users_username_unique ON users ((lower(username)));
CREATE UNIQUE INDEX users_email_unique ON users ((lower(email))) WHERE email IS NOT NULL;
CREATE INDEX users_created_at_id_idx ON users (created_at DESC, id DESC);
CREATE INDEX users_status_idx ON users (status);

CREATE TABLE system_configs (
    key varchar(128) PRIMARY KEY,
    value jsonb NOT NULL,
    value_type varchar(16) NOT NULL,
    is_public boolean NOT NULL DEFAULT false,
    description varchar(255) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT system_configs_key_format CHECK (key ~ '^[a-z][a-z0-9_.-]{1,127}$'),
    CONSTRAINT system_configs_value_type CHECK (value_type IN ('string', 'integer', 'boolean', 'duration', 'string_list'))
);

CREATE INDEX system_configs_public_key_idx ON system_configs (key) WHERE is_public = true;
