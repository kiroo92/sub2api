ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_routing_mode_check;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_routing_mode_check CHECK(routing_mode='fixed_group' OR (routing_mode IN ('all_subscriptions','team') AND group_id IS NULL));
CREATE TABLE teams (
    id BIGSERIAL PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK(status IN ('active','paused','dissolving')),
    group_ids JSONB NOT NULL DEFAULT '[]',
    daily_limit NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(daily_limit >= 0),
    weekly_limit NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(weekly_limit >= 0),
    monthly_limit NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(monthly_limit >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE team_members (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    daily_limit NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(daily_limit >= 0),
    weekly_limit NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(weekly_limit >= 0),
    monthly_limit NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK(monthly_limit >= 0),
    daily_used NUMERIC(20,8) NOT NULL DEFAULT 0,
    weekly_used NUMERIC(20,8) NOT NULL DEFAULT 0,
    monthly_used NUMERIC(20,8) NOT NULL DEFAULT 0,
    total_used NUMERIC(20,8) NOT NULL DEFAULT 0,
    daily_start TIMESTAMPTZ,
    weekly_start TIMESTAMPTZ,
    monthly_start TIMESTAMPTZ,
    UNIQUE(team_id,user_id)
);
CREATE UNIQUE INDEX team_members_one_team_per_user ON team_members(user_id) WHERE active;
CREATE TABLE team_invitations (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    email VARCHAR(320) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','accepted','revoked')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX team_invitations_pending_email ON team_invitations(team_id,email) WHERE status='pending';
CREATE TABLE team_api_keys (
    api_key_id BIGINT PRIMARY KEY REFERENCES api_keys(id) ON DELETE CASCADE,
    member_id BIGINT NOT NULL REFERENCES team_members(id),
    revoked BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE TABLE team_requests (
    id VARCHAR(64) PRIMARY KEY,
    member_id BIGINT NOT NULL REFERENCES team_members(id),
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
