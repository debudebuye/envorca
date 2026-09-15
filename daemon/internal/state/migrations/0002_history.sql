CREATE TABLE IF NOT EXISTS diagnostics (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    performed_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    overall_status TEXT NOT NULL,
    payload       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_diagnostics_performed_at ON diagnostics(performed_at);

CREATE TABLE IF NOT EXISTS repair_history (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    performed_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    action_id     TEXT NOT NULL,
    component_id  TEXT NOT NULL,
    success       INTEGER NOT NULL,
    result        TEXT NOT NULL,
    verification  TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_repair_history_performed_at ON repair_history(performed_at);