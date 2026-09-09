ALTER TABLE lottery_config ADD COLUMN IF NOT EXISTS turnstile_site_key TEXT NOT NULL DEFAULT '';
ALTER TABLE lottery_config ADD COLUMN IF NOT EXISTS turnstile_secret_key TEXT NOT NULL DEFAULT '';
