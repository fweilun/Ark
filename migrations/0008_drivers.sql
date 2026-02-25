-- README: Creates the drivers table for the driver module (depends on users and vehicles).

CREATE TABLE IF NOT EXISTS drivers (
    driver_id      TEXT PRIMARY KEY REFERENCES users(user_id),
    license_number TEXT NOT NULL UNIQUE,
    vehicle_id     BIGINT REFERENCES vehicles(vehicle_id),
    rating         DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    status         TEXT NOT NULL DEFAULT 'offline' CHECK (status IN ('available', 'on_trip', 'offline')),
    onboarded_at   TIMESTAMP NOT NULL DEFAULT NOW()
);
