package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type luckyDrawRepository struct{ db *sql.DB }

func NewLuckyDrawRepository(db *sql.DB) service.LuckyDrawRepository {
	return &luckyDrawRepository{db: db}
}

const luckyConfigColumns = `enabled,version,recharge_per_draw,guarantee_after,prizes`
const luckyRecordColumns = `id,request_id,config_version,prize_index,prize_name,prize_amount,guaranteed,created_at`

func scanLuckyConfig(row lotteryScanner) (service.LuckyDrawConfig, error) {
	var c service.LuckyDrawConfig
	var prizes []byte
	err := row.Scan(&c.Enabled, &c.Version, &c.RechargePerDraw, &c.GuaranteeAfter, &prizes)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(prizes, &c.Prizes)
	return c, err
}
func scanLuckyRecord(row lotteryScanner) (*service.LuckyDrawRecord, error) {
	var d service.LuckyDrawRecord
	err := row.Scan(&d.ID, &d.RequestID, &d.ConfigVersion, &d.PrizeIndex, &d.PrizeName, &d.PrizeAmount, &d.Guaranteed, &d.CreatedAt)
	return &d, err
}

func (r *luckyDrawRepository) Configure(ctx context.Context, c service.LuckyDrawConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	previous, err := scanLuckyConfig(tx.QueryRowContext(ctx, `SELECT `+luckyConfigColumns+` FROM lucky_draw_config WHERE id=1 FOR UPDATE`))
	if err != nil {
		return err
	}
	if previous.Version != c.Version {
		return service.ErrLuckyDrawChanged
	}
	// A fixed denominator preserves earned and already consumed lifetime chances.
	if previous.RechargePerDraw != c.RechargePerDraw {
		var used bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM lucky_draw_records)`).Scan(&used); err != nil {
			return err
		}
		if used {
			return service.ErrLuckyDrawThresholdLocked
		}
	}
	prizes, err := json.Marshal(c.Prizes)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE lucky_draw_config SET enabled=$1,version=version+1,recharge_per_draw=$2,guarantee_after=$3,prizes=$4,updated_at=NOW() WHERE id=1`, c.Enabled, lotteryMoney(c.RechargePerDraw), c.GuaranteeAfter, string(prizes))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *luckyDrawRepository) Snapshot(ctx context.Context, userID int64) (*service.LuckyDrawSnapshot, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	out := &service.LuckyDrawSnapshot{History: []service.LuckyDrawRecord{}}
	out.Config, err = scanLuckyConfig(tx.QueryRowContext(ctx, `SELECT `+luckyConfigColumns+` FROM lucky_draw_config WHERE id=1`))
	if err != nil {
		return nil, err
	}
	if userID > 0 {
		err = tx.QueryRowContext(ctx, `SELECT u.total_recharged,COALESCE(s.draws_used,0),COALESCE(s.misses,0),GREATEST(0,FLOOR(GREATEST(u.total_recharged,0)/$2::numeric)-COALESCE(s.draws_used,0))::bigint FROM users u LEFT JOIN lucky_draw_users s ON s.user_id=u.id WHERE u.id=$1 AND u.deleted_at IS NULL`, userID, lotteryMoney(out.Config.RechargePerDraw)).Scan(&out.TotalRecharged, &out.DrawsUsed, &out.Misses, &out.Chances)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		if err != nil {
			return nil, err
		}
		rows, err := tx.QueryContext(ctx, `SELECT `+luckyRecordColumns+` FROM lucky_draw_records WHERE user_id=$1 ORDER BY id DESC LIMIT 20`, userID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			d, scanErr := scanLuckyRecord(rows)
			if scanErr != nil {
				_ = rows.Close()
				return nil, scanErr
			}
			out.History = append(out.History, *d)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *luckyDrawRepository) Draw(ctx context.Context, userID int64, requestID string, version int64, pick service.LuckyDrawPicker) (*service.LuckyDrawRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Config lock is shared by all draws and config updates, including across instances.
	c, err := scanLuckyConfig(tx.QueryRowContext(ctx, `SELECT `+luckyConfigColumns+` FROM lucky_draw_config WHERE id=1 FOR UPDATE`))
	if err != nil {
		return nil, err
	}
	previous, err := scanLuckyRecord(tx.QueryRowContext(ctx, `SELECT `+luckyRecordColumns+` FROM lucky_draw_records WHERE user_id=$1 AND request_id=$2`, userID, requestID))
	if err == nil {
		return previous, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if !c.Enabled {
		return nil, service.ErrLuckyDrawPaused
	}
	if c.Version != version {
		return nil, service.ErrLuckyDrawChanged
	}
	var earned int64
	var status string
	err = tx.QueryRowContext(ctx, `SELECT FLOOR(GREATEST(total_recharged,0)/$2::numeric)::bigint,status FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID, lotteryMoney(c.RechargePerDraw)).Scan(&earned, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != service.StatusActive {
		return nil, service.ErrLuckyDrawNoChances
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO lucky_draw_users (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, userID); err != nil {
		return nil, err
	}
	var used int64
	var misses int
	if err = tx.QueryRowContext(ctx, `SELECT draws_used,misses FROM lucky_draw_users WHERE user_id=$1 FOR UPDATE`, userID).Scan(&used, &misses); err != nil {
		return nil, err
	}
	if used >= earned {
		return nil, service.ErrLuckyDrawNoChances
	}
	guaranteed := c.GuaranteeAfter > 0 && misses >= c.GuaranteeAfter-1
	index, err := pick(c.Prizes, guaranteed)
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(c.Prizes) || (guaranteed && c.Prizes[index].Amount <= 0) {
		return nil, fmt.Errorf("invalid lucky draw picker result: %d", index)
	}
	prize := c.Prizes[index]
	rules, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	result, err := scanLuckyRecord(tx.QueryRowContext(ctx, `INSERT INTO lucky_draw_records (user_id,request_id,config_version,prize_index,prize_name,prize_amount,guaranteed,rules) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+luckyRecordColumns, userID, requestID, c.Version, index, prize.Name, lotteryMoney(prize.Amount), guaranteed, string(rules)))
	if err != nil {
		return nil, err
	}
	// Misses can grow indefinitely while guarantees are disabled; cap the stored counter.
	if misses < 1000 {
		misses++
	}
	if prize.Amount > 0 {
		misses = 0
		if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$2::numeric,updated_at=NOW() WHERE id=$1`, userID, lotteryMoney(prize.Amount)); err != nil {
			return nil, err
		}
		// Promotion awards must not increment total_recharged. Persist cache invalidation.
		if _, err = tx.ExecContext(ctx, `INSERT INTO auth_cache_invalidation_outbox (cache_key) SELECT encode(sha256(convert_to(key,'UTF8')),'hex') FROM api_keys WHERE user_id=$1 AND deleted_at IS NULL AND key<>''`, userID); err != nil {
			return nil, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE lucky_draw_users SET draws_used=draws_used+1,misses=$2 WHERE user_id=$1`, userID, misses); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
