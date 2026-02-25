-- README: Creates the users table for the user module.

CREATE TABLE IF NOT EXISTS users (
    user_id    TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    phone      TEXT NOT NULL,
    user_type  TEXT NOT NULL CHECK (user_type IN ('rider', 'driver')),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
