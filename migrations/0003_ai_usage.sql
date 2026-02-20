-- README: Adds ai_usage table for per-user monthly token tracking (lazy reset pattern).

CREATE TABLE IF NOT EXISTS ai_usage (
    uid VARCHAR(255) PRIMARY KEY,
    tokens_remaining INT NOT NULL DEFAULT 100,
    last_reset_month VARCHAR(7) NOT NULL -- e.g. "2026-02"
);
