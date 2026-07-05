-- Application settings, editable at runtime from the Settings page. Bootstrap
-- values (database path, encryption key) are provided via env/flags and are
-- deliberately NOT stored here.
CREATE TABLE settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
