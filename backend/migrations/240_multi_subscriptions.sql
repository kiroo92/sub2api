ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT -1;

WITH ranked AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at, id) - 1 AS position
    FROM user_subscriptions
    WHERE deleted_at IS NULL
)
UPDATE user_subscriptions AS subscriptions
SET sort_order = ranked.position
FROM ranked
WHERE subscriptions.id = ranked.id AND subscriptions.sort_order = -1;

ALTER TABLE user_subscriptions ALTER COLUMN sort_order SET DEFAULT 0;

DROP INDEX IF EXISTS user_subscriptions_user_group_unique_active;
ALTER TABLE user_subscriptions DROP CONSTRAINT IF EXISTS user_subscriptions_user_id_group_id_key;

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_sort
    ON user_subscriptions(user_id, sort_order, id)
    WHERE deleted_at IS NULL;

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS routing_mode VARCHAR(24) NOT NULL DEFAULT 'fixed_group';

ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_routing_mode_check;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_routing_mode_check
    CHECK (
        routing_mode = 'fixed_group'
        OR (routing_mode = 'all_subscriptions' AND group_id IS NULL)
    );
