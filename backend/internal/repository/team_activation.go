package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *teamRepository) ActivationInvitations(ctx context.Context) ([]service.TeamActivationInvitation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,email,expires_at,created_at FROM team_activation_invitations WHERE status='pending' AND expires_at>NOW() ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.TeamActivationInvitation{}
	for rows.Next() {
		var inv service.TeamActivationInvitation
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (r *teamRepository) InviteActivation(ctx context.Context, adminID int64, email, hash string) (*service.TeamActivationInvitation, error) {
	var out service.TeamActivationInvitation
	err := r.transaction(ctx, func(tx *sql.Tx) error {
		var admin, member bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL)`, adminID).Scan(&admin); err != nil {
			return err
		}
		if !admin {
			return service.ErrTeamForbidden
		}
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM team_members m JOIN users u ON u.id=m.user_id WHERE lower(u.email)=$1 AND m.active AND u.deleted_at IS NULL)`, email).Scan(&member); err != nil {
			return err
		}
		if member {
			return service.ErrTeamConflict
		}
		return tx.QueryRowContext(ctx, `INSERT INTO team_activation_invitations(email,token_hash,created_by,expires_at) VALUES($1,$2,$3,NOW()+INTERVAL '7 days') ON CONFLICT(email) WHERE status='pending' DO UPDATE SET token_hash=EXCLUDED.token_hash,created_by=EXCLUDED.created_by,expires_at=EXCLUDED.expires_at RETURNING id,email,expires_at,created_at`, email, hash, adminID).Scan(&out.ID, &out.Email, &out.ExpiresAt, &out.CreatedAt)
	})
	return &out, err
}

func (r *teamRepository) RevokeActivation(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE team_activation_invitations SET status='revoked' WHERE id=$1 AND status='pending'`, id)
	return teamAffected(res, err)
}
