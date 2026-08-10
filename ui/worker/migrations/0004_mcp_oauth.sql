ALTER TABLE configs ADD COLUMN last_change_id TEXT;

CREATE TABLE oauth_clients (
  client_id       TEXT PRIMARY KEY,
  client_name     TEXT NOT NULL,
  redirect_uris   TEXT NOT NULL,
  created_at      INTEGER NOT NULL
);

CREATE INDEX idx_oauth_clients_created
  ON oauth_clients(created_at DESC);

CREATE TABLE oauth_authorization_requests (
  request_id      TEXT PRIMARY KEY,
  client_id       TEXT NOT NULL,
  redirect_uri    TEXT NOT NULL,
  state           TEXT,
  code_challenge  TEXT NOT NULL,
  resource        TEXT NOT NULL,
  scope           TEXT NOT NULL,
  csrf_hash       TEXT NOT NULL,
  created_at      INTEGER NOT NULL,
  expires_at      INTEGER NOT NULL,
  decided_at      INTEGER,
  status          TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'denied', 'expired')),
  FOREIGN KEY (client_id) REFERENCES oauth_clients(client_id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_authorization_requests_expiry
  ON oauth_authorization_requests(status, expires_at);

CREATE TABLE oauth_grants (
  grant_id         TEXT PRIMARY KEY,
  client_id        TEXT NOT NULL,
  access_identity  TEXT NOT NULL,
  scope            TEXT NOT NULL,
  created_at       INTEGER NOT NULL,
  last_used_at     INTEGER,
  revoked_at       INTEGER,
  FOREIGN KEY (client_id) REFERENCES oauth_clients(client_id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_grants_client
  ON oauth_grants(client_id, created_at DESC);

CREATE TABLE oauth_codes (
  code_hash       TEXT PRIMARY KEY,
  grant_id        TEXT NOT NULL,
  client_id       TEXT NOT NULL,
  redirect_uri    TEXT NOT NULL,
  code_challenge  TEXT NOT NULL,
  resource        TEXT NOT NULL,
  expires_at      INTEGER NOT NULL,
  used_at         INTEGER,
  FOREIGN KEY (grant_id) REFERENCES oauth_grants(grant_id) ON DELETE CASCADE,
  FOREIGN KEY (client_id) REFERENCES oauth_clients(client_id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_codes_expiry ON oauth_codes(expires_at);

CREATE TABLE oauth_tokens (
  token_hash  TEXT PRIMARY KEY,
  grant_id    TEXT NOT NULL,
  kind        TEXT NOT NULL CHECK (kind IN ('access', 'refresh')),
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL,
  revoked_at  INTEGER,
  FOREIGN KEY (grant_id) REFERENCES oauth_grants(grant_id) ON DELETE CASCADE
);

CREATE INDEX idx_oauth_tokens_grant_kind
  ON oauth_tokens(grant_id, kind, expires_at);

CREATE TABLE config_changes (
  change_id         TEXT PRIMARY KEY,
  config_name       TEXT NOT NULL,
  actor_kind        TEXT NOT NULL CHECK (actor_kind IN ('mcp', 'human')),
  actor_id          TEXT NOT NULL,
  actor_label       TEXT NOT NULL,
  before_hash       TEXT NOT NULL,
  after_hash        TEXT NOT NULL,
  patch             TEXT NOT NULL,
  diff              TEXT NOT NULL,
  before_doc        TEXT NOT NULL,
  after_doc         TEXT NOT NULL,
  source_change_id  TEXT,
  created_at        INTEGER NOT NULL,
  FOREIGN KEY (config_name) REFERENCES configs(name) ON DELETE CASCADE
);

CREATE INDEX idx_config_changes_config_created
  ON config_changes(config_name, created_at DESC);

-- Agent writes set last_change_id and append their audit record in the same
-- D1 batch.  If the optimistic hash update matched no row, this trigger aborts
-- the batch rather than allowing an orphaned audit event.
CREATE TRIGGER config_changes_require_committed_config
BEFORE INSERT ON config_changes
WHEN NOT EXISTS (
  SELECT 1 FROM configs
  WHERE name = NEW.config_name
    AND hash = NEW.after_hash
    AND last_change_id = NEW.change_id
)
BEGIN
  SELECT RAISE(ABORT, 'config change was not committed');
END;
