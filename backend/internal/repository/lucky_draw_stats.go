package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// paid_at fixes eligibility at actual payment time; completion is required even
// if the payment callback arrives late. Pending refunds do not remove chances.
const luckyRechargeEligible = `p.order_type='balance' AND p.paid_at >= $3 AND p.completed_at IS NOT NULL AND p.status IN ('COMPLETED','REFUND_REQUESTED','REFUND_PENDING','REFUNDING','REFUND_FAILED','PARTIALLY_REFUNDED','REFUNDED')`

// Use actual payment numbers, with no exchange or balance-recharge multiplier.
// RefundAmount is in credited-balance units: apply its proportion to payment.
const luckyRechargeNet = `GREATEST(p.pay_amount,0) * CASE WHEN p.status='REFUNDED' THEN 0 WHEN p.status='PARTIALLY_REFUNDED' THEN GREATEST(0,1-p.refund_amount/NULLIF(p.amount,0)) ELSE 1 END`

const luckyStatsCTE = `WITH recharge AS (
 SELECT p.user_id,COALESCE(SUM(` + luckyRechargeNet + `),0) AS amount FROM payment_orders p WHERE ` + luckyRechargeEligible + ` GROUP BY p.user_id
), awards AS (
 SELECT user_id,COUNT(*) FILTER(WHERE prize_amount>0) AS wins,SUM(prize_amount) AS prize_amount FROM lucky_draw_records GROUP BY user_id
), stats AS (
 SELECT u.id AS user_id,u.email,COALESCE(r.amount,0) AS recharge_amount,
 FLOOR(COALESCE(r.amount,0)/$2::numeric)::bigint AS chances_earned,
 COALESCE(s.draws_used,0) AS draws_used,COALESCE(s.manual_chances,0) AS manual_chances,
 GREATEST(0,FLOOR(COALESCE(r.amount,0)/$2::numeric)+COALESCE(s.manual_chances,0)-COALESCE(s.draws_used,0))::bigint AS chances,
 COALESCE(a.wins,0) AS wins,COALESCE(a.prize_amount,0) AS prize_amount
 FROM users u LEFT JOIN recharge r ON r.user_id=u.id LEFT JOIN lucky_draw_users s ON s.user_id=u.id LEFT JOIN awards a ON a.user_id=u.id
 WHERE u.deleted_at IS NULL
) `

func (r *luckyDrawRepository) AdminStats(ctx context.Context, search string, page int) (*service.LuckyDrawAdminStats, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	c, err := scanLuckyConfig(tx.QueryRowContext(ctx, `SELECT `+luckyConfigColumns+` FROM lucky_draw_config WHERE id=1`))
	if err != nil {
		return nil, err
	}
	out := &service.LuckyDrawAdminStats{RechargeStartsAt: c.RechargeStartsAt, Users: []service.LuckyDrawUserStats{}, Prizes: []service.LuckyDrawPrizeStats{}, Grants: []service.LuckyDrawGrant{}, Page: page, PageSize: 20}
	// $1 is reserved for an exact/substring search; totals remain site-wide.
	args := []any{search, lotteryMoney(c.RechargePerDraw), c.RechargeStartsAt}
	t := &out.Summary
	err = tx.QueryRowContext(ctx, luckyStatsCTE+`SELECT COALESCE(SUM(recharge_amount),0),COALESCE(SUM(chances_earned),0),COALESCE(SUM(manual_chances),0),COALESCE(SUM(draws_used),0),COALESCE(SUM(chances),0),COALESCE(SUM(wins),0),COALESCE(SUM(prize_amount),0),COUNT(*) FILTER(WHERE $1='' OR strpos(lower(email),lower($1))>0 OR user_id::text=$1) FROM stats`, args...).Scan(&t.RechargeAmount, &t.ChancesEarned, &t.ManualChances, &t.DrawsUsed, &t.Chances, &t.Wins, &t.PrizeAmount, &out.Total)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, luckyStatsCTE+`SELECT user_id,email,recharge_amount,chances_earned,manual_chances,draws_used,chances,wins,prize_amount FROM stats WHERE $1='' OR strpos(lower(email),lower($1))>0 OR user_id::text=$1 ORDER BY user_id DESC LIMIT 20 OFFSET $4`, append(args, (page-1)*20)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var u service.LuckyDrawUserStats
		if err = rows.Scan(&u.UserID, &u.Email, &u.RechargeAmount, &u.ChancesEarned, &u.ManualChances, &u.DrawsUsed, &u.Chances, &u.Wins, &u.PrizeAmount); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out.Users = append(out.Users, u)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT prize_name,prize_amount,COUNT(*),SUM(prize_amount) FROM lucky_draw_records GROUP BY prize_name,prize_amount ORDER BY prize_amount DESC,prize_name`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p service.LuckyDrawPrizeStats
		if err = rows.Scan(&p.Name, &p.Amount, &p.Draws, &p.TotalAwarded); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out.Prizes = append(out.Prizes, p)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT `+luckyGrantColumns+` FROM lucky_draw_grants ORDER BY id DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		g, e := scanLuckyGrant(rows)
		if e != nil {
			_ = rows.Close()
			return nil, e
		}
		out.Grants = append(out.Grants, *g)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}
