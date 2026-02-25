-- README: Creates the ratings table for the rating module.

CREATE TABLE IF NOT EXISTS ratings (
    rating_id     BIGSERIAL PRIMARY KEY,
    trip_id       TEXT NOT NULL REFERENCES orders(id),
    rider_rating  INT CHECK (rider_rating BETWEEN 1 AND 5),
    driver_rating INT CHECK (driver_rating BETWEEN 1 AND 5),
    comments      TEXT NOT NULL DEFAULT ''
);
