CREATE TABLE configs (
  name        TEXT PRIMARY KEY,
  doc         TEXT NOT NULL,
  toml        TEXT NOT NULL,
  hash        TEXT NOT NULL,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);

CREATE INDEX idx_configs_updated ON configs(updated_at DESC);
