-- README: Adds scheduled-order columns and indexes to the orders table.

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS order_type TEXT NOT NULL DEFAULT 'instant',
    ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS schedule_window_mins INT,
    ADD COLUMN IF NOT EXISTS cancel_deadline_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS incentive_bonus BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP;

-- Drivers query available scheduled orders by time window.
CREATE INDEX IF NOT EXISTS idx_orders_scheduled_open
    ON orders (status, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_orders_scheduled_available
    ON orders (scheduled_at)
    WHERE status = 'scheduled';

-- Passengers and drivers query their own order history.
CREATE INDEX IF NOT EXISTS idx_orders_passenger_time
    ON orders (passenger_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_driver_time
    ON orders (driver_id, created_at DESC);
