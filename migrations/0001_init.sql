-- README: Initial MVP schema for orders, order events, locations, and pricing rates.

CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    passenger_id TEXT NOT NULL,
    driver_id TEXT,
    status TEXT NOT NULL,
    status_version INT NOT NULL DEFAULT 0,
    pickup_lat DOUBLE PRECISION,
    pickup_lng DOUBLE PRECISION,
    dropoff_lat DOUBLE PRECISION,
    dropoff_lng DOUBLE PRECISION,
    ride_type TEXT,
    estimated_fee BIGINT,
    actual_fee BIGINT,
    created_at TIMESTAMP NOT NULL,
    matched_at TIMESTAMP,
    accepted_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    cancelled_at TIMESTAMP,
    cancellation_reason TEXT
);

CREATE TABLE IF NOT EXISTS order_state_events (
    id BIGSERIAL PRIMARY KEY,
    order_id TEXT NOT NULL,
    from_status TEXT NOT NULL,
    to_status TEXT NOT NULL,
    actor_type TEXT NOT NULL,
    actor_id TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS location_snapshots (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    user_type TEXT NOT NULL,
    lat DOUBLE PRECISION NOT NULL,
    lng DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_location_user_time ON location_snapshots (user_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS pricing_rates (
    ride_type TEXT PRIMARY KEY,
    base_fare BIGINT NOT NULL,
    per_km BIGINT NOT NULL,
    currency TEXT NOT NULL
);
