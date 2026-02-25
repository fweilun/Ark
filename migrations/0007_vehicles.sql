-- README: Creates the vehicles table for the vehicle module.

CREATE TABLE IF NOT EXISTS vehicles (
    vehicle_id        BIGSERIAL PRIMARY KEY,
    driver_id         TEXT REFERENCES users(user_id),
    make              TEXT NOT NULL,
    model             TEXT NOT NULL,
    license_plate     TEXT NOT NULL UNIQUE,
    capacity          INT NOT NULL DEFAULT 4,
    vehicle_type      TEXT NOT NULL CHECK (vehicle_type IN ('sedan', 'suv', 'bike')),
    registration_date DATE NOT NULL
);
