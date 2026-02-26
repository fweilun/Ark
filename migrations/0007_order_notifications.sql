-- README: Tracks driver notification history and cooldown per order for the matching module.

CREATE TABLE IF NOT EXISTS order_notifications (
    order_id           TEXT PRIMARY KEY REFERENCES orders(id) ON DELETE CASCADE,
    notify_count       INT NOT NULL DEFAULT 0,
    last_notified_at   TIMESTAMP NOT NULL,
    next_notifiable_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_order_notifications_notifiable
    ON order_notifications (next_notifiable_at);
