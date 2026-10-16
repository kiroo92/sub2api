-- Instant balance lottery; prizes and draw results retain immutable snapshots.
CREATE TABLE IF NOT EXISTS lucky_draw_config (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version BIGINT NOT NULL DEFAULT 1,
    recharge_per_draw NUMERIC(20,2) NOT NULL DEFAULT 50 CHECK (recharge_per_draw > 0),
    guarantee_after INTEGER NOT NULL DEFAULT 10 CHECK (guarantee_after BETWEEN 0 AND 1000),
    prizes JSONB NOT NULL DEFAULT '[{"name":"$0.50","amount":0.5,"weight":4000},{"name":"$1","amount":1,"weight":2000},{"name":"$5","amount":5,"weight":500},{"name":"$20","amount":20,"weight":100},{"name":"谢谢参与","amount":0,"weight":3400}]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO lucky_draw_config (id) VALUES (1) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS lucky_draw_users (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    draws_used BIGINT NOT NULL DEFAULT 0 CHECK (draws_used >= 0),
    misses INTEGER NOT NULL DEFAULT 0 CHECK (misses >= 0)
);
CREATE TABLE IF NOT EXISTS lucky_draw_records (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id UUID NOT NULL,
    config_version BIGINT NOT NULL,
    prize_index INTEGER NOT NULL,
    prize_name VARCHAR(80) NOT NULL,
    prize_amount NUMERIC(20,2) NOT NULL CHECK (prize_amount >= 0),
    guaranteed BOOLEAN NOT NULL,
    rules JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, request_id)
);
CREATE INDEX IF NOT EXISTS lucky_draw_user_history ON lucky_draw_records (user_id, id DESC);
