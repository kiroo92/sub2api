package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type lotteryRepository struct{ db *sql.DB }

func NewLotteryRepository(db *sql.DB) service.LotteryRepository { return &lotteryRepository{db: db} }

type lotteryScanner interface{ Scan(...any) error }

const lotteryConfigColumns = `enabled, prize_amount, winner_count, participant_target, min_recharge, turnstile_site_key, turnstile_secret_key`
const lotteryRoundColumns = `id, prize_amount, winner_count, participant_target, min_recharge, participant_count, winners_drawn, status, created_at, drawn_at`

func scanLotteryConfig(row lotteryScanner) (service.LotteryConfig, error) {
	var c service.LotteryConfig
	err := row.Scan(&c.Enabled, &c.PrizeAmount, &c.WinnerCount, &c.ParticipantTarget, &c.MinRecharge, &c.TurnstileSiteKey, &c.TurnstileSecretKey)
	return c, err
}
func scanLotteryRound(row lotteryScanner) (*service.LotteryRound, error) {
	var r service.LotteryRound
	err := row.Scan(&r.ID, &r.PrizeAmount, &r.WinnerCount, &r.ParticipantTarget, &r.MinRecharge, &r.ParticipantCount, &r.WinnersDrawn, &r.Status, &r.CreatedAt, &r.DrawnAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}
func lotteryMoney(value float64) string { return strconv.FormatFloat(value, 'f', 2, 64) }

func createLotteryRound(ctx context.Context, tx *sql.Tx, c service.LotteryConfig) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO lottery_rounds (prize_amount,winner_count,participant_target,min_recharge) VALUES ($1,$2,$3,$4)`, lotteryMoney(c.PrizeAmount), c.WinnerCount, c.ParticipantTarget, lotteryMoney(c.MinRecharge))
	return err
}

func (r *lotteryRepository) Configure(ctx context.Context, c service.LotteryConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// All joins and config edits share this lock across every application instance.
	if _, err = scanLotteryConfig(tx.QueryRowContext(ctx, `SELECT `+lotteryConfigColumns+` FROM lottery_config WHERE id=1 FOR UPDATE`)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE lottery_config SET enabled=$1,prize_amount=$2,winner_count=$3,participant_target=$4,min_recharge=$5,turnstile_site_key=$6,turnstile_secret_key=COALESCE(NULLIF($7,''),turnstile_secret_key),updated_at=NOW() WHERE id=1`, c.Enabled, lotteryMoney(c.PrizeAmount), c.WinnerCount, c.ParticipantTarget, lotteryMoney(c.MinRecharge), c.TurnstileSiteKey, c.TurnstileSecretKey); err != nil {
		return err
	}
	if c.Enabled {
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM lottery_rounds WHERE status='open')`).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if err = createLotteryRound(ctx, tx, c); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (r *lotteryRepository) Snapshot(ctx context.Context, userID int64) (*service.LotterySnapshot, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	out := &service.LotterySnapshot{RecentWinners: []service.LotteryWin{}, MyWins: []service.LotteryWin{}, RecentRounds: []service.LotteryRound{}}
	out.Config, err = scanLotteryConfig(tx.QueryRowContext(ctx, `SELECT `+lotteryConfigColumns+` FROM lottery_config WHERE id=1`))
	if err != nil {
		return nil, err
	}
	out.Current, err = scanLotteryRound(tx.QueryRowContext(ctx, `SELECT `+lotteryRoundColumns+` FROM lottery_rounds WHERE status='open'`))
	if err != nil {
		return nil, err
	}
	if userID > 0 {
		var status string
		err = tx.QueryRowContext(ctx, `SELECT total_recharged,status FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&out.TotalRecharged, &status)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		if err != nil {
			return nil, err
		}
		if out.Current != nil {
			out.Eligible = status == service.StatusActive && out.TotalRecharged >= out.Current.MinRecharge
			if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM lottery_entries WHERE round_id=$1 AND user_id=$2)`, out.Current.ID, userID).Scan(&out.Joined); err != nil {
				return nil, err
			}
		}
	}
	out.RecentWinners, err = lotteryWins(ctx, tx, 0)
	if err != nil {
		return nil, err
	}
	if userID > 0 {
		out.MyWins, err = lotteryWins(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+lotteryRoundColumns+` FROM lottery_rounds ORDER BY id DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		round, scanErr := scanLotteryRound(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		out.RecentRounds = append(out.RecentRounds, *round)
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

func lotteryWins(ctx context.Context, tx *sql.Tx, userID int64) ([]service.LotteryWin, error) {
	query := `SELECT e.round_id,u.email,e.prize_amount,e.awarded_at FROM lottery_entries e JOIN users u ON u.id=e.user_id WHERE e.prize_amount>0`
	args := []any{}
	if userID > 0 {
		query += ` AND e.user_id=$1`
		args = append(args, userID)
	}
	query += ` ORDER BY e.awarded_at DESC,e.round_id DESC,e.user_id LIMIT 20`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	wins := []service.LotteryWin{}
	for rows.Next() {
		var w service.LotteryWin
		if err = rows.Scan(&w.RoundID, &w.Email, &w.PrizeAmount, &w.AwardedAt); err != nil {
			return nil, err
		}
		wins = append(wins, w)
	}
	return wins, rows.Err()
}

func (r *lotteryRepository) Join(ctx context.Context, userID, roundID int64, pick service.LotteryWinnerPicker) (*service.LotteryJoinResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	config, err := scanLotteryConfig(tx.QueryRowContext(ctx, `SELECT `+lotteryConfigColumns+` FROM lottery_config WHERE id=1 FOR UPDATE`))
	if err != nil {
		return nil, err
	}
	result := &service.LotteryJoinResult{RoundID: roundID}
	// A retry of a completed round must never enroll the user into the next round.
	var previousStatus string
	err = tx.QueryRowContext(ctx, `SELECT r.status FROM lottery_entries e JOIN lottery_rounds r ON r.id=e.round_id WHERE e.round_id=$1 AND e.user_id=$2`, roundID, userID).Scan(&previousStatus)
	if err == nil {
		result.AlreadyJoined = true
		result.Drawn = previousStatus == "drawn"
		return result, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if !config.Enabled {
		return nil, service.ErrLotteryPaused
	}
	round, err := scanLotteryRound(tx.QueryRowContext(ctx, `SELECT `+lotteryRoundColumns+` FROM lottery_rounds WHERE status='open'`))
	if err != nil {
		return nil, err
	}
	if round == nil || round.ID != roundID {
		return nil, service.ErrLotteryRoundChanged
	}
	var total float64
	var status string
	err = tx.QueryRowContext(ctx, `SELECT total_recharged,status FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&total, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != service.StatusActive || total < round.MinRecharge {
		return nil, service.ErrLotteryNotEligible
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO lottery_entries (round_id,user_id) VALUES ($1,$2)`, roundID, userID); err != nil {
		return nil, err
	}
	// Count persisted entries instead of trusting a cached counter (including hard deletions).
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM lottery_entries WHERE round_id=$1`, roundID).Scan(&count); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE lottery_rounds SET participant_count=$2 WHERE id=$1`, roundID, count); err != nil {
		return nil, err
	}
	if count >= round.ParticipantTarget {
		ids, err := lotteryEntrants(ctx, tx, roundID)
		if err != nil {
			return nil, err
		}
		winners, err := pick(ids, round.WinnerCount)
		if err != nil {
			return nil, err
		}
		// Stable balance lock order avoids deadlocks with other multi-user operations.
		sort.Slice(winners, func(i, j int) bool { return winners[i] < winners[j] })
		for _, id := range winners {
			if err = awardLotteryPrize(ctx, tx, round, id); err != nil {
				return nil, err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE lottery_rounds SET status='drawn',winners_drawn=$2,drawn_at=NOW() WHERE id=$1`, roundID, len(winners)); err != nil {
			return nil, err
		}
		if err = createLotteryRound(ctx, tx, config); err != nil {
			return nil, err
		}
		result.Drawn = true
		result.WinnerIDs = winners
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func lotteryEntrants(ctx context.Context, tx *sql.Tx, roundID int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM lottery_entries WHERE round_id=$1 ORDER BY user_id`, roundID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func awardLotteryPrize(ctx context.Context, tx *sql.Tx, round *service.LotteryRound, userID int64) error {
	amount := lotteryMoney(round.PrizeAmount)
	res, err := tx.ExecContext(ctx, `UPDATE lottery_entries SET prize_amount=$3,awarded_at=NOW() WHERE round_id=$1 AND user_id=$2 AND prize_amount=0`, round.ID, userID, amount)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("lottery award entry missing or already paid: round=%d user=%d", round.ID, userID)
	}
	// Promotional winnings are not recharge: never increase total_recharged.
	res, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$2::numeric,updated_at=NOW() WHERE id=$1`, userID, amount)
	if err != nil {
		return err
	}
	n, err = res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrUserNotFound
	}
	// Durable invalidation survives process termination after the credit commits.
	_, err = tx.ExecContext(ctx, `INSERT INTO auth_cache_invalidation_outbox (cache_key) SELECT encode(sha256(convert_to(key,'UTF8')),'hex') FROM api_keys WHERE user_id=$1 AND deleted_at IS NULL AND key<>''`, userID)
	return err
}
