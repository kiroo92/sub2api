-- Independent package commerce; deliberately does not modify legacy subscriptions.
CREATE TABLE package_plans (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id),
    terms JSONB NOT NULL,
    for_sale BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE package_group_buys (
    id BIGSERIAL PRIMARY KEY,
    creator_id BIGINT NOT NULL REFERENCES users(id),
    plan_id BIGINT NOT NULL REFERENCES package_plans(id),
    plan_snapshot JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','open','settled')),
    paid_count INTEGER NOT NULL DEFAULT 0 CHECK (paid_count >= 0),
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    settled_at TIMESTAMPTZ,
    final_members INTEGER,
    final_quota_usd NUMERIC(20,8),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX package_group_buys_open ON package_group_buys(ends_at,id) WHERE status='open';
CREATE UNIQUE INDEX package_group_buys_draft ON package_group_buys(creator_id,plan_id) WHERE status='draft';
CREATE TABLE package_orders (
    order_id BIGINT PRIMARY KEY REFERENCES payment_orders(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    plan_id BIGINT NOT NULL REFERENCES package_plans(id),
    plan_snapshot JSONB NOT NULL,
    group_buy_id BIGINT REFERENCES package_group_buys(id),
    paid_at TIMESTAMPTZ,
    UNIQUE(group_buy_id,user_id)
);
CREATE TABLE user_packages (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    order_id BIGINT NOT NULL UNIQUE REFERENCES package_orders(order_id),
    plan_snapshot JSONB NOT NULL,
    group_buy_id BIGINT REFERENCES package_group_buys(id),
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at > starts_at),
    sort_order BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX user_packages_ordered ON user_packages(user_id,sort_order,id);
CREATE INDEX user_packages_group_buy ON user_packages(group_buy_id);
CREATE TABLE package_periods (
    id BIGSERIAL PRIMARY KEY,
    package_id BIGINT NOT NULL REFERENCES user_packages(id),
    period_index INTEGER NOT NULL CHECK (period_index BETWEEN 1 AND 4),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL CHECK (ends_at > starts_at),
    quota_usd NUMERIC(20,8) NOT NULL CHECK (quota_usd > 0),
    used_usd NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (used_usd >= 0),
    UNIQUE(package_id,period_index)
);
ALTER TABLE api_keys ADD COLUMN routing_mode VARCHAR(20) NOT NULL DEFAULT 'fixed_group';
ALTER TABLE api_keys ADD CONSTRAINT api_keys_package_mode CHECK (
    routing_mode IN ('fixed_group','all_packages') AND (routing_mode <> 'all_packages' OR group_id IS NULL)
);
