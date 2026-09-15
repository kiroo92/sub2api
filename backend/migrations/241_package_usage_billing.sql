CREATE TABLE package_usage_billing (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    package_id BIGINT NOT NULL REFERENCES user_packages(id),
    period_id BIGINT NOT NULL REFERENCES package_periods(id),
    request_fingerprint TEXT NOT NULL,
    command JSONB NOT NULL,
    applied BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    applied_at TIMESTAMPTZ,
    UNIQUE (request_id, api_key_id)
);
CREATE INDEX package_usage_billing_pending_owner_idx ON package_usage_billing(user_id) WHERE NOT applied;
