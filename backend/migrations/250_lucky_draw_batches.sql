CREATE TABLE IF NOT EXISTS lucky_draw_batches (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id UUID NOT NULL,
    draw_count INTEGER NOT NULL CHECK (draw_count IN (1,5,10)),
    result JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, request_id)
);
