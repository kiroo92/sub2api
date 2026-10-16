ALTER TABLE lucky_draw_users ADD COLUMN IF NOT EXISTS manual_chances BIGINT NOT NULL DEFAULT 0 CHECK (manual_chances >= 0);
CREATE TABLE IF NOT EXISTS lucky_draw_grants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    admin_id BIGINT NOT NULL,
    request_id UUID NOT NULL,
    chances INTEGER NOT NULL CHECK (chances BETWEEN 1 AND 10000),
    note VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(admin_id,request_id)
);
CREATE INDEX IF NOT EXISTS lucky_draw_grants_user ON lucky_draw_grants(user_id,id DESC);
