-- An empty allowlist preserves unrestricted access for existing members.
ALTER TABLE team_members ADD COLUMN IF NOT EXISTS allowed_models JSONB NOT NULL DEFAULT '[]'::jsonb;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'team_member_models_array' AND conrelid = 'team_members'::regclass
    ) THEN
        ALTER TABLE team_members ADD CONSTRAINT team_member_models_array CHECK (jsonb_typeof(allowed_models) = 'array');
    END IF;
END $$;
