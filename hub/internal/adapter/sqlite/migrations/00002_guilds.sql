-- +goose Up
CREATE TABLE files (
  id INTEGER PRIMARY KEY,
  owner_id INTEGER REFERENCES users (id) ON DELETE SET NULL,
  purpose TEXT NOT NULL CHECK (purpose IN ('avatar', 'guild_icon', 'attachment')),
  sha256 BLOB NOT NULL,
  size INTEGER NOT NULL,
  mime TEXT NOT NULL,
  original_name TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  storage_key TEXT NOT NULL,
  created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX files_storage_key ON files (storage_key);

CREATE TABLE guilds (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  icon_file_id INTEGER REFERENCES files (id) ON DELETE SET NULL,
  owner_id INTEGER NOT NULL REFERENCES users (id),
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX guilds_owner ON guilds (owner_id);

CREATE TABLE guild_members (
  guild_id INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
  joined_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (guild_id, user_id)
) STRICT, WITHOUT ROWID;

CREATE INDEX guild_members_user ON guild_members (user_id);

CREATE TABLE guild_bans (
  guild_id INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  banned_by INTEGER REFERENCES users (id) ON DELETE SET NULL,
  reason TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  PRIMARY KEY (guild_id, user_id)
) STRICT, WITHOUT ROWID;

CREATE TABLE invites (
  code TEXT PRIMARY KEY,
  guild_id INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
  created_by INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  max_uses INTEGER NOT NULL DEFAULT 0,
  uses INTEGER NOT NULL DEFAULT 0,
  expires_at INTEGER,
  revoked_at INTEGER,
  created_at INTEGER NOT NULL
) STRICT, WITHOUT ROWID;

CREATE INDEX invites_guild ON invites (guild_id);

CREATE TABLE channels (
  id INTEGER PRIMARY KEY,
  guild_id INTEGER REFERENCES guilds (id) ON DELETE CASCADE,
  parent_id INTEGER REFERENCES channels (id) ON DELETE SET NULL,
  type TEXT NOT NULL CHECK (type IN ('category', 'text', 'voice', 'thread', 'dm')),
  name TEXT NOT NULL,
  topic TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL DEFAULT 0,
  user_limit INTEGER NOT NULL DEFAULT 0,
  drawing_allowed INTEGER NOT NULL DEFAULT 1,
  max_share_quality TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX channels_guild ON channels (guild_id);

CREATE TABLE audit_log (
  id INTEGER PRIMARY KEY,
  guild_id INTEGER NOT NULL REFERENCES guilds (id) ON DELETE CASCADE,
  actor_id INTEGER REFERENCES users (id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL DEFAULT '',
  data_json TEXT NOT NULL DEFAULT '{}',
  created_at INTEGER NOT NULL
) STRICT;

CREATE INDEX audit_log_guild ON audit_log (guild_id, id);

-- +goose Down
DROP TABLE audit_log;
DROP TABLE channels;
DROP TABLE invites;
DROP TABLE guild_bans;
DROP TABLE guild_members;
DROP TABLE guilds;
DROP TABLE files;
