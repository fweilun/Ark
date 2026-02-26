-- README: Creates the users table for natural persons (riders and drivers).

CREATE TABLE IF NOT EXISTS users (
    user_id    TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    phone      TEXT NOT NULL DEFAULT '',
    user_type  TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);
