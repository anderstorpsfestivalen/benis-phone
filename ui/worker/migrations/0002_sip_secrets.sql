ALTER TABLE configs ADD COLUMN secret_revision INTEGER NOT NULL DEFAULT 0;

CREATE TABLE sip_secrets (
  config_name   TEXT NOT NULL,
  connection_id TEXT NOT NULL,
  version       INTEGER NOT NULL,
  iv            TEXT NOT NULL,
  ciphertext    TEXT NOT NULL,
  updated_at    INTEGER NOT NULL,
  PRIMARY KEY (config_name, connection_id),
  FOREIGN KEY (config_name) REFERENCES configs(name) ON DELETE CASCADE
);

CREATE INDEX idx_sip_secrets_config ON sip_secrets(config_name);
