-- Lottery settings and immutable per-round rules. Disabled until configured by an admin.
CREATE TABLE IF NOT EXISTS lottery_config (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    prize_amount NUMERIC(20,2) NOT NULL DEFAULT 5 CHECK (prize_amount > 0 AND prize_amount <= 10000),
    winner_count INTEGER NOT NULL DEFAULT 6 CHECK (winner_count BETWEEN 1 AND 100),
    participant_target INTEGER NOT NULL DEFAULT 60 CHECK (participant_target BETWEEN 2 AND 10000),
    min_recharge NUMERIC(20,2) NOT NULL DEFAULT 50 CHECK (min_recharge >= 0 AND min_recharge <= 1000000),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (winner_count <= participant_target)
);
INSERT INTO lottery_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING;
CREATE TABLE IF NOT EXISTS lottery_rounds (
    id BIGSERIAL PRIMARY KEY,
    prize_amount NUMERIC(20,2) NOT NULL CHECK (prize_amount > 0),
    winner_count INTEGER NOT NULL CHECK (winner_count > 0),
    participant_target INTEGER NOT NULL CHECK (participant_target >= winner_count),
    min_recharge NUMERIC(20,2) NOT NULL CHECK (min_recharge >= 0),
    participant_count INTEGER NOT NULL DEFAULT 0,
    winners_drawn INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(16) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'drawn')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    drawn_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS lottery_one_open_round ON lottery_rounds (status) WHERE status = 'open';
CREATE TABLE IF NOT EXISTS lottery_entries (
    round_id BIGINT NOT NULL REFERENCES lottery_rounds(id),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prize_amount NUMERIC(20,2) NOT NULL DEFAULT 0 CHECK (prize_amount >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    awarded_at TIMESTAMPTZ,
    PRIMARY KEY (round_id, user_id)
);
CREATE INDEX IF NOT EXISTS lottery_entries_user ON lottery_entries(user_id, round_id DESC);
CREATE INDEX IF NOT EXISTS lottery_recent_winners ON lottery_entries(awarded_at DESC, round_id DESC) WHERE prize_amount > 0;
