-- README: Creates the plans table for the planner module (AI secretary trip planning).

CREATE TABLE IF NOT EXISTS plans (
    id          TEXT PRIMARY KEY,
    passenger_id TEXT NOT NULL,
    origin      TEXT NOT NULL DEFAULT '',
    destination TEXT NOT NULL,
    pickup_at   TIMESTAMP NOT NULL,
    status      TEXT NOT NULL DEFAULT 'draft',
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_plans_passenger ON plans (passenger_id, pickup_at ASC);
