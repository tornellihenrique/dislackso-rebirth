-- +goose Up
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  email TEXT NOT NULL UNIQUE COLLATE NOCASE,
  email_verified_at INTEGER,
  password_hash TEXT NOT NULL,
  display_name TEXT NOT NULL,
  avatar_file_id INTEGER,
  status_pref TEXT NOT NULL DEFAULT 'online'
    CHECK (status_pref IN ('online', 'idle', 'dnd', 'invisible')),
  custom_status TEXT NOT NULL DEFAULT '',
  is_instance_admin INTEGER NOT NULL DEFAULT 0,
  disabled_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  deleted_at INTEGER
) STRICT;

CREATE TABLE email_tokens (
  id INTEGER PRIMARY KEY,
  user_id INTEGER REFERENCES users (id) ON DELETE CASCADE,
  purpose TEXT NOT NULL
    CHECK (purpose IN ('verify', 'reset', 'change_email', 'instance_invite')),
  token_hash BLOB NOT NULL UNIQUE,
  payload TEXT NOT NULL DEFAULT '',
  expires_at INTEGER NOT NULL,
  used_at INTEGER,
  created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX email_tokens_user_purpose ON email_tokens (user_id, purpose);

CREATE TABLE sessions (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash BLOB NOT NULL UNIQUE,
  created_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  user_agent TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT '',
  revoked_at INTEGER
) STRICT;

CREATE INDEX sessions_user ON sessions (user_id);

CREATE TABLE instance_settings (
  key TEXT PRIMARY KEY,
  value_json TEXT NOT NULL,
  updated_at INTEGER NOT NULL
) STRICT;

-- +goose Down
DROP TABLE instance_settings;
DROP TABLE sessions;
DROP TABLE email_tokens;
DROP TABLE users;
