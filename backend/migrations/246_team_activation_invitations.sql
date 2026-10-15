-- Enterprise teams can only be created with an administrator-issued invitation.
CREATE TABLE IF NOT EXISTS team_activation_invitations (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(320) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'revoked')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS team_activation_pending_email
    ON team_activation_invitations(email) WHERE status = 'pending';
