CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    uuid TEXT NOT NULL UNIQUE,
    reality_short_id_id TEXT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    comment TEXT NULL,
    traffic_limit_bytes BIGINT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS reality_keysets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    public_key TEXT NOT NULL,
    private_key_secret_path TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL,
    rotated_at TIMESTAMP NULL,
    comment TEXT NULL
);

CREATE TABLE IF NOT EXISTS reality_short_ids (
    id TEXT PRIMARY KEY,
    keyset_id TEXT NOT NULL,
    short_id TEXT NOT NULL,
    assigned_user_id TEXT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP NULL,
    FOREIGN KEY (keyset_id) REFERENCES reality_keysets(id) ON DELETE RESTRICT,
    FOREIGN KEY (assigned_user_id) REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE (keyset_id, short_id)
);

CREATE TABLE IF NOT EXISTS rendered_configs (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    version BIGINT NOT NULL,
    path TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    created_by TEXT NULL,
    UNIQUE (kind, version)
);

CREATE TABLE IF NOT EXISTS audit_events (
    id TEXT PRIMARY KEY,
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL,
    subject_type TEXT NOT NULL,
    subject_id TEXT NULL,
    payload_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS mtproto_secrets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    secret_file_path TEXT NOT NULL,
    secret_sha256 TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL,
    rotated_at TIMESTAMP NULL,
    comment TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active, revoked_at);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_reality_short_id_id ON users(reality_short_id_id) WHERE reality_short_id_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_reality_keysets_active ON reality_keysets(is_active, created_at);
CREATE INDEX IF NOT EXISTS idx_reality_short_ids_keyset_active ON reality_short_ids(keyset_id, is_active, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_reality_short_ids_assigned_user ON reality_short_ids(assigned_user_id) WHERE assigned_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_rendered_configs_kind_version ON rendered_configs(kind, version DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_created_at ON audit_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_subject ON audit_events(subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_mtproto_secrets_active ON mtproto_secrets(is_active, created_at DESC);
