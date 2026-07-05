-- Initial Bothan schema. Timestamps are UTC; booleans are INTEGER 0/1.

-- hosts to monitor
CREATE TABLE hosts (
  id              INTEGER PRIMARY KEY,
  hostname        TEXT    NOT NULL UNIQUE,
  enabled         INTEGER NOT NULL DEFAULT 1,
  publish         INTEGER NOT NULL DEFAULT 0,   -- SSL Labs publish flag; 0 = private (default), 1 = public
  ignore_mismatch INTEGER NOT NULL DEFAULT 0,
  from_cache      INTEGER NOT NULL DEFAULT 0,
  max_age_hours   INTEGER,                       -- used only when from_cache=1
  notes           TEXT,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- reusable schedules
CREATE TABLE schedules (
  id         INTEGER PRIMARY KEY,
  name       TEXT NOT NULL UNIQUE,
  spec       TEXT NOT NULL,        -- cron expr, @descriptor, or friendly text (normalized)
  enabled    INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE host_schedules (
  host_id     INTEGER NOT NULL REFERENCES hosts(id)     ON DELETE CASCADE,
  schedule_id INTEGER NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
  PRIMARY KEY (host_id, schedule_id)
);

-- notification channels (config stored AES-256-GCM encrypted)
CREATE TABLE channels (
  id               INTEGER PRIMARY KEY,
  name             TEXT NOT NULL UNIQUE,
  type             TEXT NOT NULL,   -- shoutrrr | whatsapp_greenapi | whatsapp_multidevice
  config_encrypted BLOB,             -- null when imported without secrets
  needs_credentials INTEGER NOT NULL DEFAULT 0, -- set on secret-less import; blocks use until re-entered
  enabled          INTEGER NOT NULL DEFAULT 1,
  created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE host_channels (
  host_id    INTEGER NOT NULL REFERENCES hosts(id)    ON DELETE CASCADE,
  channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  PRIMARY KEY (host_id, channel_id)
);

-- notification rules (host_id NULL = global default applied to all hosts)
CREATE TABLE rules (
  id             INTEGER PRIMARY KEY,
  host_id        INTEGER REFERENCES hosts(id) ON DELETE CASCADE,
  name           TEXT NOT NULL,
  condition_type TEXT NOT NULL,   -- grade_below | grade_changed | grade_downgraded |
                                  -- grade_improved | cert_expiry | scan_failed |
                                  -- vuln_detected | scan_completed
  threshold_grade TEXT,           -- for grade_below (e.g. 'A')
  expiry_days     INTEGER,        -- for cert_expiry
  enabled         INTEGER NOT NULL DEFAULT 1,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- scan runs
CREATE TABLE scans (
  id               INTEGER PRIMARY KEY,
  host_id          INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
  status           TEXT NOT NULL,      -- pending | running | ready | error
  trigger          TEXT NOT NULL,      -- manual | api | schedule:<name>
  overall_grade    TEXT,               -- lowest endpoint grade
  engine_version   TEXT,
  criteria_version TEXT,
  error_message    TEXT,
  raw_json         BLOB,               -- full SSL Labs Host object (for compare/detail)
  started_at       TIMESTAMP,
  completed_at     TIMESTAMP,
  created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_scans_host_created ON scans(host_id, created_at DESC);

-- per-endpoint results
CREATE TABLE scan_endpoints (
  id            INTEGER PRIMARY KEY,
  scan_id       INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
  ip_address    TEXT NOT NULL,
  server_name   TEXT,
  grade         TEXT,
  grade_trust_ignored TEXT,
  has_warnings  INTEGER,
  is_exceptional INTEGER,
  status_message TEXT,
  cert_not_after TIMESTAMP,           -- earliest cert expiry for this endpoint
  progress      INTEGER
);
CREATE INDEX idx_scan_endpoints_scan ON scan_endpoints(scan_id);

-- optional auth
CREATE TABLE users (
  id            INTEGER PRIMARY KEY,
  username      TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,        -- argon2id
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY,
  name         TEXT NOT NULL,
  token_hash   TEXT NOT NULL UNIQUE,  -- store hash only; show plaintext once at creation
  scopes       TEXT NOT NULL,         -- csv of: read,write,admin
  last_used_at TIMESTAMP,
  expires_at   TIMESTAMP,
  created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
