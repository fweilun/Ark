-- README: Adds user_fcm_tokens table for Firebase Cloud Messaging device registration.

CREATE TABLE IF NOT EXISTS user_fcm_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,               -- 對應 types.ID 的字串形式
    fcm_token TEXT NOT NULL,
    platform VARCHAR(20) NOT NULL,              -- 'ios', 'android', 'web'
    device_id VARCHAR(128),                     -- 前端提供的裝置唯一識別碼（可選）
    last_seen_at TIMESTAMPTZ DEFAULT NOW(),     -- 最近一次使用此 Token 的時間
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT unique_user_token UNIQUE (user_id, fcm_token)
);

-- 索引：依 user_id 快速查詢
CREATE INDEX idx_user_fcm_tokens_user_id ON user_fcm_tokens(user_id);

-- 索引：供清理過期裝置使用
CREATE INDEX idx_user_fcm_tokens_last_seen ON user_fcm_tokens(last_seen_at);
