ALTER TABLE team_requests ADD COLUMN closed BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE team_requests ADD COLUMN unresolved BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE team_billing_pending (
    request_id TEXT NOT NULL,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    admission_id VARCHAR(64) NOT NULL REFERENCES team_requests(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    command JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(request_id, api_key_id)
);
CREATE INDEX team_billing_pending_admission ON team_billing_pending(admission_id);
