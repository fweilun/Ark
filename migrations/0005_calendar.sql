-- README: Creates tables for the calendar module (events and schedules).

CREATE TABLE IF NOT EXISTS calendar_events (
    id          TEXT PRIMARY KEY,
    "from"      TIMESTAMP NOT NULL,
    "to"        TIMESTAMP NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS calendar_schedules (
    uid         TEXT NOT NULL,
    event_id    TEXT NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    tied_order  TEXT,
    PRIMARY KEY (uid, event_id)
);

CREATE INDEX IF NOT EXISTS idx_calendar_schedules_uid ON calendar_schedules (uid);
