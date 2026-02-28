-- README: Friendships table for the relation module.

CREATE TABLE IF NOT EXISTS friendships (
    id          BIGSERIAL PRIMARY KEY,
    user_id     TEXT      NOT NULL,
    friend_id   TEXT      NOT NULL,
    status      SMALLINT  NOT NULL DEFAULT 0,
    group_id    INT,
    remark      VARCHAR(50),
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Composite unique index prevents duplicate requests in the same direction.
CREATE UNIQUE INDEX IF NOT EXISTS uidx_friendships_pair
    ON friendships (user_id, friend_id);

-- Unique index to prevent duplicate friendships for the same unordered pair.
CREATE UNIQUE INDEX IF NOT EXISTS uidx_friendships_unordered_pair
    ON friendships (LEAST(user_id, friend_id), GREATEST(user_id, friend_id));
-- Index to quickly find all requests for a given user (by status).
CREATE INDEX IF NOT EXISTS idx_friendships_user_status
    ON friendships (user_id, status);

CREATE INDEX IF NOT EXISTS idx_friendships_friend_status
    ON friendships (friend_id, status);
