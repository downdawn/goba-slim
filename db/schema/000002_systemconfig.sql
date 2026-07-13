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

INSERT INTO schema_migrations (version, name) VALUES (2, 'systemconfig');
