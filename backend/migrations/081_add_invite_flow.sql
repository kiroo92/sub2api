CREATE TABLE IF NOT EXISTS invite_codes (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code VARCHAR(32) NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS invite_codes_user_id_key ON invite_codes(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS invite_codes_code_key ON invite_codes(code);

CREATE TABLE IF NOT EXISTS invite_bindings (
  id BIGSERIAL PRIMARY KEY,
  inviter_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  invitee_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  invite_code_id BIGINT NOT NULL REFERENCES invite_codes(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS invite_bindings_invitee_user_id_key ON invite_bindings(invitee_user_id);
CREATE INDEX IF NOT EXISTS invite_bindings_inviter_user_id_idx ON invite_bindings(inviter_user_id);
CREATE INDEX IF NOT EXISTS invite_bindings_invite_code_id_idx ON invite_bindings(invite_code_id);
