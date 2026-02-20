-- README: Adds ai_usage table for per-user monthly token tracking (lazy reset pattern).

CREATE TABLE IF NOT EXISTS ai_usage (
    uid TEXT PRIMARY KEY,
    tokens_remaining INT NOT NULL DEFAULT 100,
    last_reset_month TEXT NOT NULL DEFAULT to_char(now(), 'YYYY-MM') -- e.g. "2026-02"
);
