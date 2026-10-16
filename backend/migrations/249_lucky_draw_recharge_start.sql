-- A fixed campaign boundary, never a moving "today" on server restart.
ALTER TABLE lucky_draw_config ADD COLUMN IF NOT EXISTS recharge_starts_at TIMESTAMPTZ NOT NULL DEFAULT '2026-10-16 00:00:00+08';
CREATE INDEX IF NOT EXISTS idx_lucky_draw_paid_recharges ON payment_orders (user_id, paid_at) WHERE order_type='balance' AND completed_at IS NOT NULL;
-- Initial campaign defaults may change only before any draw has been committed.
UPDATE lucky_draw_config SET recharge_per_draw=5, guarantee_after=10,
    prizes='[{"name":"¥20","amount":20,"weight":5},{"name":"¥5","amount":5,"weight":50},{"name":"¥2","amount":2,"weight":200},{"name":"¥1","amount":1,"weight":700},{"name":"¥0.5","amount":0.5,"weight":2000},{"name":"谢谢参与","amount":0,"weight":7045}]',
    version=version+1
WHERE id=1 AND recharge_per_draw=50
AND prizes='[{"name":"$0.50","amount":0.5,"weight":4000},{"name":"$1","amount":1,"weight":2000},{"name":"$5","amount":5,"weight":500},{"name":"$20","amount":20,"weight":100},{"name":"谢谢参与","amount":0,"weight":3400}]'::jsonb
AND NOT EXISTS (SELECT 1 FROM lucky_draw_records);
