CREATE TABLE IF NOT EXISTS subscription_discount_codes (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9_-]{1,64}$'),
    discount_type VARCHAR(20) NOT NULL CHECK (discount_type IN ('percentage', 'fixed_amount')),
    discount_value DECIMAL(20,2) NOT NULL CHECK (discount_value > 0 AND discount_value < 'Infinity'::numeric),
    plan_ids JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(plan_ids) = 'array'),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    max_uses INTEGER NOT NULL DEFAULT 0 CHECK (max_uses >= 0),
    per_user_limit INTEGER NOT NULL DEFAULT 1 CHECK (per_user_limit >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (discount_type <> 'percentage' OR discount_value < 100)
);

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS discount_code_id BIGINT,
    ADD COLUMN IF NOT EXISTS discount_state VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS discount_snapshot JSONB;

CREATE INDEX IF NOT EXISTS payment_orders_discount_code_id_discount_state_user_id
    ON payment_orders (discount_code_id, discount_state, user_id);

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'payment_orders_discount_consistent' AND conrelid = 'payment_orders'::regclass) THEN
        ALTER TABLE payment_orders ADD CONSTRAINT payment_orders_discount_consistent CHECK (
            (discount_code_id IS NULL AND discount_state = '' AND discount_snapshot IS NULL)
            OR (discount_code_id IS NOT NULL AND discount_code_id > 0 AND discount_state IN ('creating', 'reserved', 'consumed', 'released') AND discount_snapshot IS NOT NULL)
        );
    END IF;
END $$;
