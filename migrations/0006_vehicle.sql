-- README: Creates the vehicles table for the vehicle module.

CREATE TABLE IF NOT EXISTS vehicles (
    id                TEXT PRIMARY KEY,
    driver_id         TEXT NOT NULL UNIQUE,
    make              TEXT NOT NULL,
    model             TEXT NOT NULL,
    license_plate     TEXT NOT NULL,
    capacity          INT  NOT NULL CHECK (capacity > 0),
    vehicle_type      TEXT NOT NULL CHECK (vehicle_type IN ('sedan', 'suv', 'bike')),
    registration_date TEXT,
    created_at        TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vehicles_driver_id ON vehicles (driver_id);
