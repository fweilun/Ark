-- README: Separates order-event links into their own table, allowing multiple orders per event.

-- Remove tied_order from calendar_schedules (schedules now track user-event attendance only)
ALTER TABLE calendar_schedules DROP COLUMN IF EXISTS tied_order;

-- New table for order-event associations (many orders can be linked to one event)
CREATE TABLE IF NOT EXISTS calendar_order_events (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL REFERENCES calendar_events(id) ON DELETE CASCADE,
    order_id    TEXT NOT NULL,
    uid         TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_calendar_order_events_event ON calendar_order_events (event_id);
CREATE INDEX IF NOT EXISTS idx_calendar_order_events_uid ON calendar_order_events (uid);
