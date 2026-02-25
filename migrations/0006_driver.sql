-- README: Drivers table for driver-specific attributes. driver_id is a foreign key to users.user_id.

CREATE TABLE IF NOT EXISTS drivers (
    driver_id    TEXT PRIMARY KEY,
    license_number TEXT NOT NULL,
    vehicle_id   TEXT,
    rating       DOUBLE PRECISION NOT NULL DEFAULT 5.0,
    status       TEXT NOT NULL DEFAULT 'available',
    onboarded_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_drivers_status ON drivers (status);
