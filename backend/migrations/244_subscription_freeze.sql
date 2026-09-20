ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS frozen_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS frozen_duration_us BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS admin_assignment_key VARCHAR(64),
    ADD COLUMN IF NOT EXISTS admin_assignment_fingerprint VARCHAR(64);

CREATE UNIQUE INDEX IF NOT EXISTS user_subscriptions_admin_assignment_key_key
    ON user_subscriptions (admin_assignment_key);

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'user_subscriptions_frozen_duration_valid' AND conrelid = 'user_subscriptions'::regclass) THEN
        ALTER TABLE user_subscriptions ADD CONSTRAINT user_subscriptions_frozen_duration_valid
            CHECK (frozen_duration_us >= 0 AND frozen_duration_us <= 9223372036854775);
    END IF;
END $$;
