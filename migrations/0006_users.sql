-- README: Users table for natural persons (riders and drivers).

CREATE TABLE IF NOT EXISTS users (
    user_id    SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL,
    phone      TEXT NOT NULL DEFAULT '',
    user_type  TEXT NOT NULL CHECK (user_type IN ('rider', 'driver')),
    created_at TIMESTAMP NOT NULL
);
