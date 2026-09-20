CREATE TABLE IF NOT EXISTS invoice_requests (
 id BIGSERIAL PRIMARY KEY,
 user_id BIGINT NOT NULL REFERENCES users(id),
 status VARCHAR(24) NOT NULL DEFAULT 'awaiting_payment' CHECK (status IN ('awaiting_payment','pending','issued','cancelled')),
 tax_id VARCHAR(64) NOT NULL, title VARCHAR(200) NOT NULL, email VARCHAR(254) NOT NULL, remarks VARCHAR(1000) NOT NULL DEFAULT '',
 currency VARCHAR(3) NOT NULL DEFAULT 'CNY' CHECK (currency = 'CNY'),
 base_amount DECIMAL(20,2) NOT NULL CHECK (base_amount > 0 AND base_amount < 'Infinity'::numeric),
 service_fee DECIMAL(20,2) NOT NULL CHECK (service_fee > 0 AND service_fee < 'Infinity'::numeric),
 total_amount DECIMAL(20,2) NOT NULL, net_amount DECIMAL(20,2) NOT NULL, tax_amount DECIMAL(20,2) NOT NULL,
 item_name VARCHAR(200) NOT NULL, tax_rate DECIMAL(10,4) NOT NULL CHECK (tax_rate >= 0 AND tax_rate <= 100),
 fee_type VARCHAR(20) NOT NULL CHECK (fee_type IN ('fixed','percentage')), fee_value DECIMAL(20,2) NOT NULL CHECK (fee_value > 0), fee_upper_amount DECIMAL(20,2),
 operation_key VARCHAR(64) NOT NULL UNIQUE, fingerprint VARCHAR(64) NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL, submitted_at TIMESTAMPTZ, issued_at TIMESTAMPTZ, issued_by BIGINT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK (total_amount = base_amount + service_fee AND net_amount >= 0 AND tax_amount >= 0 AND total_amount = net_amount + tax_amount)
);
CREATE INDEX IF NOT EXISTS invoice_requests_user_id_created_at ON invoice_requests(user_id, created_at);
CREATE INDEX IF NOT EXISTS invoice_requests_status_expires_at ON invoice_requests(status, expires_at);
CREATE TABLE IF NOT EXISTS invoice_request_orders (
 id BIGSERIAL PRIMARY KEY,
 invoice_request_id BIGINT NOT NULL REFERENCES invoice_requests(id),
 order_id BIGINT NOT NULL REFERENCES payment_orders(id),
 order_no VARCHAR(64) NOT NULL, order_type VARCHAR(20) NOT NULL, name VARCHAR(200) NOT NULL,
 amount DECIMAL(20,2) NOT NULL CHECK (amount > 0 AND amount < 'Infinity'::numeric), released_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS invoice_request_orders_invoice_request_id ON invoice_request_orders(invoice_request_id);
CREATE UNIQUE INDEX IF NOT EXISTS invoice_request_orders_order_id ON invoice_request_orders(order_id) WHERE released_at IS NULL;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS invoice_request_id BIGINT REFERENCES invoice_requests(id);
CREATE UNIQUE INDEX IF NOT EXISTS payment_orders_invoice_request_id ON payment_orders(invoice_request_id);
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'payment_orders_invoice_type' AND conrelid = 'payment_orders'::regclass) THEN
  ALTER TABLE payment_orders ADD CONSTRAINT payment_orders_invoice_type CHECK (
   (invoice_request_id IS NULL AND order_type <> 'invoice_fee') OR (invoice_request_id IS NOT NULL AND order_type = 'invoice_fee' AND discount_code_id IS NULL AND plan_id IS NULL AND subscription_group_id IS NULL AND fee_rate = 0)
  );
 END IF;
END $$;
