package repository

import (
	"context"
	"database/sql"
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const luckyGrantColumns = `id,user_id,admin_id,chances,note,created_at`

func scanLuckyGrant(row lotteryScanner) (*service.LuckyDrawGrant, error) {
	var g service.LuckyDrawGrant
	err := row.Scan(&g.ID, &g.UserID, &g.AdminID, &g.Chances, &g.Note, &g.CreatedAt)
	return &g, err
}
func (r *luckyDrawRepository) GrantChances(ctx context.Context, adminID, userID int64, requestID string, count int, note string) (*service.LuckyDrawGrant, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT id FROM lucky_draw_config WHERE id=1 FOR UPDATE`); err != nil {
		return nil, err
	}
	var oldUser int64
	var oldCount int
	var oldNote string
	err = tx.QueryRowContext(ctx, `SELECT user_id,chances,note FROM lucky_draw_grants WHERE admin_id=$1 AND request_id=$2`, adminID, requestID).Scan(&oldUser, &oldCount, &oldNote)
	if err == nil {
		if oldUser != userID || oldCount != count || oldNote != note {
			return nil, infraerrors.Conflict("LUCKY_DRAW_REQUEST_CONFLICT", "同一请求不能更改赠送内容")
		}
		grant, err := scanLuckyGrant(tx.QueryRowContext(ctx, `SELECT `+luckyGrantColumns+` FROM lucky_draw_grants WHERE admin_id=$1 AND request_id=$2`, adminID, requestID))
		if err != nil {
			return nil, err
		}
		return grant, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != service.StatusActive {
		return nil, infraerrors.BadRequest("LUCKY_DRAW_USER_INACTIVE", "仅可给正常状态用户赠送次数")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO lucky_draw_users(user_id,manual_chances) VALUES ($1,$2) ON CONFLICT(user_id) DO UPDATE SET manual_chances=lucky_draw_users.manual_chances+EXCLUDED.manual_chances`, userID, count); err != nil {
		return nil, err
	}
	grant, err := scanLuckyGrant(tx.QueryRowContext(ctx, `INSERT INTO lucky_draw_grants(user_id,admin_id,request_id,chances,note) VALUES ($1,$2,$3,$4,$5) RETURNING `+luckyGrantColumns, userID, adminID, requestID, count, note))
	if err != nil {
		return nil, err
	}
	return grant, tx.Commit()
}
