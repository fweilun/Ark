-- README: Location snapshots table for driver/passenger latest positions.

CREATE TABLE IF NOT EXISTS location_snapshots (
    id          BIGSERIAL PRIMARY KEY,
    user_id     TEXT        NOT NULL,
    user_type   TEXT        NOT NULL,
    lat         FLOAT8      NOT NULL,
    lng         FLOAT8      NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    UNIQUE (user_id, user_type)
);
