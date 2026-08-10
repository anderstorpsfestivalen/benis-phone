ALTER TABLE configs ADD COLUMN registration_id TEXT;

UPDATE configs
SET registration_id = lower(hex(randomblob(4))) || '-' ||
                      lower(hex(randomblob(2))) || '-' ||
                      '4' || substr(lower(hex(randomblob(2))), 2) || '-' ||
                      substr('89ab', abs(random()) % 4 + 1, 1) ||
                      substr(lower(hex(randomblob(2))), 2) || '-' ||
                      lower(hex(randomblob(6)))
WHERE registration_id IS NULL;

CREATE UNIQUE INDEX idx_configs_registration_id
  ON configs(registration_id);

CREATE TABLE credential_bundles (
  config_name TEXT PRIMARY KEY,
  version     INTEGER NOT NULL,
  iv          TEXT NOT NULL,
  ciphertext  TEXT NOT NULL,
  updated_at  INTEGER NOT NULL,
  FOREIGN KEY (config_name) REFERENCES configs(name) ON DELETE CASCADE
);

CREATE TABLE bridge_enrollments (
  request_id    TEXT PRIMARY KEY,
  config_name   TEXT NOT NULL,
  public_key    TEXT NOT NULL,
  fingerprint   TEXT NOT NULL,
  hostname      TEXT NOT NULL,
  platform      TEXT NOT NULL,
  version       TEXT NOT NULL,
  created_at    INTEGER NOT NULL,
  expires_at    INTEGER NOT NULL,
  status        TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'denied', 'expired')),
  decided_at    INTEGER,
  bridge_id     TEXT,
  FOREIGN KEY (config_name) REFERENCES configs(name) ON DELETE CASCADE
);

CREATE INDEX idx_bridge_enrollments_config_status
  ON bridge_enrollments(config_name, status, created_at DESC);
CREATE UNIQUE INDEX idx_bridge_enrollments_pending_key
  ON bridge_enrollments(config_name, public_key)
  WHERE status = 'pending';

CREATE TRIGGER bridge_enrollments_pending_limit
BEFORE INSERT ON bridge_enrollments
WHEN NEW.status = 'pending' AND (
  SELECT count(*) FROM bridge_enrollments
  WHERE config_name = NEW.config_name AND status = 'pending'
) >= 5
BEGIN
  SELECT RAISE(ABORT, 'pending enrollment limit reached');
END;

CREATE TABLE bridges (
  bridge_id    TEXT PRIMARY KEY,
  config_name  TEXT NOT NULL,
  public_key   TEXT NOT NULL,
  fingerprint  TEXT NOT NULL,
  approved_at  INTEGER NOT NULL,
  last_seen    INTEGER,
  revoked_at   INTEGER,
  FOREIGN KEY (config_name) REFERENCES configs(name) ON DELETE CASCADE
);

CREATE INDEX idx_bridges_config ON bridges(config_name, approved_at DESC);
CREATE UNIQUE INDEX idx_bridges_active_key
  ON bridges(config_name, public_key)
  WHERE revoked_at IS NULL;

CREATE TABLE bridge_nonces (
  bridge_id  TEXT NOT NULL,
  nonce      TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  PRIMARY KEY (bridge_id, nonce),
  FOREIGN KEY (bridge_id) REFERENCES bridges(bridge_id) ON DELETE CASCADE
);

CREATE INDEX idx_bridge_nonces_expiry ON bridge_nonces(expires_at);
