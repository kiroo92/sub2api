package repository

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func luckyTestDB(t *testing.T) (*sql.DB, service.LuckyDrawRepository, service.LuckyDrawConfig) {
	t.Helper()
	db := lotteryTestDB(t)
	migration, err := migrations.FS.ReadFile("248_lucky_draw.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = db.Exec(string(migration))
		require.NoError(t, err)
	}
	_, err = db.Exec(`CREATE TABLE payment_orders (id BIGSERIAL PRIMARY KEY,user_id BIGINT,amount NUMERIC(20,2),pay_amount NUMERIC(20,2),refund_amount NUMERIC(20,2) NOT NULL DEFAULT 0,order_type TEXT NOT NULL DEFAULT 'balance',status TEXT NOT NULL DEFAULT 'COMPLETED',paid_at TIMESTAMPTZ,completed_at TIMESTAMPTZ)`)
	require.NoError(t, err)
	extra, err := migrations.FS.ReadFile("249_lucky_draw_recharge_start.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = db.Exec(string(extra))
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT INTO payment_orders (user_id,amount,pay_amount,paid_at,completed_at) SELECT id,100,100,'2026-10-16 01:00:00+08','2026-10-16 01:01:00+08' FROM users`)
	require.NoError(t, err)
	for _, file := range []string{"250_lucky_draw_batches.sql", "251_lucky_draw_manual_chances.sql"} {
		data, err := migrations.FS.ReadFile(file)
		require.NoError(t, err)
		for i := 0; i < 2; i++ {
			_, err = db.Exec(string(data))
			require.NoError(t, err)
		}
	}
	repo := NewLuckyDrawRepository(db)
	snapshot, err := repo.Snapshot(context.Background(), 1)
	require.NoError(t, err)
	require.False(t, snapshot.Config.Enabled)
	require.Len(t, snapshot.Config.Prizes, 6)
	require.Equal(t, 5.0, snapshot.Config.RechargePerDraw)
	require.Equal(t, "2026-10-15T16:00:00Z", snapshot.Config.RechargeStartsAt.UTC().Format(time.RFC3339))
	c := snapshot.Config
	c.Enabled = true
	c.RechargePerDraw = 50
	c.GuaranteeAfter = 3
	c.Prizes = []service.LuckyDrawPrize{{Name: "余额", Amount: 0.5, Weight: 100}, {Name: "未中奖", Weight: 9900}}
	require.NoError(t, repo.Configure(context.Background(), c))
	c.Version++
	return db, repo, c
}

func TestLuckyDrawPostgresConcurrentRetry(t *testing.T) {
	db, repo, c := luckyTestDB(t)
	ctx := context.Background()
	id := uuid.NewString()
	wins := func([]service.LuckyDrawPrize, bool) (int, error) { return 0, nil }
	results := make(chan *service.LuckyDrawRecord, 12)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, err := repo.Draw(ctx, 1, id, c.Version, wins); results <- r; errs <- err }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var first int64
	for r := range results {
		if first == 0 {
			first = r.ID
		}
		require.Equal(t, first, r.ID)
	}
	snapshot, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, snapshot.DrawsUsed)
	require.EqualValues(t, 1, snapshot.Chances)
	require.Len(t, snapshot.History, 1)
	var balance, recharge float64
	var outbox int
	require.NoError(t, db.QueryRow(`SELECT balance,total_recharged FROM users WHERE id=1`).Scan(&balance, &recharge))
	require.Equal(t, 0.5, balance)
	require.Equal(t, 100.0, recharge)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM auth_cache_invalidation_outbox`).Scan(&outbox))
	require.Equal(t, 1, outbox)
	// A response lost before a rule edit or pause still resolves to its original award.
	c.Enabled = false
	c.Prizes[0].Amount = 20
	c.RechargeStartsAt = time.Now().AddDate(-10, 0, 0)
	require.NoError(t, repo.Configure(ctx, c))
	configured, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "2026-10-15T16:00:00Z", configured.Config.RechargeStartsAt.UTC().Format(time.RFC3339), "config writes cannot alter campaign start")
	retry, err := repo.Draw(ctx, 1, id, c.Version, wins)
	require.NoError(t, err)
	require.Equal(t, 0.5, retry.PrizeAmount)
	_, err = repo.Draw(ctx, 1, uuid.NewString(), c.Version+1, wins)
	require.ErrorIs(t, err, service.ErrLuckyDrawPaused)
	c.Version++
	c.RechargePerDraw = 25
	require.ErrorIs(t, repo.Configure(ctx, c), service.ErrLuckyDrawThresholdLocked)
}

func TestLuckyDrawPostgresGuaranteeAndBoundaries(t *testing.T) {
	db, repo, c := luckyTestDB(t)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE payment_orders SET amount=149.99,pay_amount=149.99 WHERE user_id=1`)
	require.NoError(t, err)
	calls := 0
	pick := func(_ []service.LuckyDrawPrize, guaranteed bool) (int, error) {
		calls++
		if guaranteed {
			return 0, nil
		}
		return 1, nil
	}
	for i := 0; i < 2; i++ {
		d, err := repo.Draw(ctx, 1, uuid.NewString(), c.Version, pick)
		require.NoError(t, err)
		require.Zero(t, d.PrizeAmount)
		require.False(t, d.Guaranteed)
	}
	_, err = repo.Draw(ctx, 1, uuid.NewString(), c.Version, pick)
	require.ErrorIs(t, err, service.ErrLuckyDrawNoChances)
	require.Equal(t, 2, calls)
	_, err = db.Exec(`UPDATE payment_orders SET amount=150,pay_amount=150 WHERE user_id=1`)
	require.NoError(t, err)
	d, err := repo.Draw(ctx, 1, uuid.NewString(), c.Version, pick)
	require.NoError(t, err)
	require.True(t, d.Guaranteed)
	require.Equal(t, 0.5, d.PrizeAmount)
	snapshot, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, snapshot.Chances)
	require.Zero(t, snapshot.Misses)
	require.EqualValues(t, 3, snapshot.DrawsUsed)
	other, err := repo.Snapshot(ctx, 2)
	require.NoError(t, err)
	require.Empty(t, other.History)
	// Clearing unrelated usage data cannot affect independent lottery state.
	_, err = db.Exec(`CREATE TABLE usage_logs (id BIGINT); INSERT INTO usage_logs VALUES (1); DELETE FROM usage_logs`)
	require.NoError(t, err)
	after, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, snapshot, after)
}

func TestLuckyDrawPostgresLastChanceAndRollback(t *testing.T) {
	db, repo, c := luckyTestDB(t)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE payment_orders SET amount=50,pay_amount=50 WHERE user_id=1; ALTER TABLE users ADD CONSTRAINT reject_lucky_award CHECK (balance=0)`)
	require.NoError(t, err)
	wins := func([]service.LuckyDrawPrize, bool) (int, error) { return 0, nil }
	requestID := uuid.NewString()
	_, err = repo.Draw(ctx, 1, requestID, c.Version, wins)
	require.Error(t, err)
	before, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, before.Chances)
	require.Zero(t, before.DrawsUsed)
	require.Empty(t, before.History)
	_, err = db.Exec(`ALTER TABLE users DROP CONSTRAINT reject_lucky_award`)
	require.NoError(t, err)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := repo.Draw(ctx, 1, uuid.NewString(), c.Version, wins); errs <- err }()
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, service.ErrLuckyDrawNoChances)
		}
	}
	require.Equal(t, 1, successes)
	_, err = repo.Draw(ctx, 2, uuid.NewString(), c.Version-1, wins)
	require.ErrorIs(t, err, service.ErrLuckyDrawChanged)
}

func TestLuckyDrawPostgresPaymentCutoffAndStats(t *testing.T) {
	db, repo, c := luckyTestDB(t)
	ctx := context.Background()
	_, err := db.Exec(`DELETE FROM payment_orders WHERE user_id IN (1,2);
        UPDATE users SET total_recharged=99999,balance=999 WHERE id=1;
        INSERT INTO payment_orders (user_id,amount,pay_amount,paid_at,completed_at,status,order_type,refund_amount) VALUES
        (1,500,500,'2026-10-15 23:59:59.999999+08',NOW(),'COMPLETED','balance',0),
        (1,200,50,'2026-10-16 00:00:00+08',NOW(),'COMPLETED','balance',0),
        (1,200,200,'2026-10-16 01:00:00+08',NULL,'PAID','balance',0),
        (1,200,200,'2026-10-16 01:00:00+08',NOW(),'COMPLETED','subscription',0),
        (1,100,100,'2026-10-16 01:00:00+08',NOW(),'REFUNDED','balance',100),
        (1,200,100,'2026-10-16 01:00:00+08',NOW(),'PARTIALLY_REFUNDED','balance',100),
        (1,100,50,'2026-10-16 01:00:00+08',NOW(),'REFUND_REQUESTED','balance',100)`)
	require.NoError(t, err)
	snapshot, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 150.0, snapshot.TotalRecharged, "only qualifying payment numbers count; no recharge multiplier or conversion")
	require.EqualValues(t, 3, snapshot.Chances)
	empty, err := repo.Snapshot(ctx, 2)
	require.NoError(t, err)
	require.Zero(t, empty.Chances, "lifetime total alone earns no chances")
	win := func([]service.LuckyDrawPrize, bool) (int, error) { return 0, nil }
	lose := func([]service.LuckyDrawPrize, bool) (int, error) { return 1, nil }
	_, err = repo.Draw(ctx, 1, uuid.NewString(), c.Version, win)
	require.NoError(t, err)
	_, err = repo.Draw(ctx, 1, uuid.NewString(), c.Version, lose)
	require.NoError(t, err)
	stats, err := repo.AdminStats(ctx, "person1@example.com", 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Total)
	require.Len(t, stats.Users, 1)
	u := stats.Users[0]
	require.EqualValues(t, 1, u.UserID)
	require.EqualValues(t, 3, u.ChancesEarned)
	require.EqualValues(t, 2, u.DrawsUsed)
	require.EqualValues(t, 1, u.Chances)
	require.EqualValues(t, 1, u.Wins)
	require.Equal(t, 0.5, u.PrizeAmount)
	require.EqualValues(t, 129, stats.Summary.ChancesEarned)
	require.EqualValues(t, 2, stats.Summary.DrawsUsed)
	require.Equal(t, 0.5, stats.Summary.PrizeAmount)
	require.Len(t, stats.Prizes, 2)
	require.EqualValues(t, 1, stats.Prizes[0].Draws)
	all, err := repo.AdminStats(ctx, "", 1)
	require.NoError(t, err)
	require.EqualValues(t, 65, all.Total)
	require.Len(t, all.Users, 20)
	next, err := repo.AdminStats(ctx, "", 2)
	require.NoError(t, err)
	require.Len(t, next.Users, 20)
	require.NotEqual(t, all.Users[0].UserID, next.Users[0].UserID)
	// Confirmed refund reduces net eligible recharge; it does not refund draws.
	_, err = db.Exec(`UPDATE payment_orders SET status='REFUNDED' WHERE user_id=1 AND status='REFUND_REQUESTED'`)
	require.NoError(t, err)
	after, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 100.0, after.TotalRecharged)
	require.Zero(t, after.Chances)
	require.EqualValues(t, 2, after.DrawsUsed)
}

func TestLuckyDrawPostgresBatchAndManualGrant(t *testing.T) {
	db, repo, c := luckyTestDB(t)
	ctx := context.Background()
	_, err := db.Exec(`DELETE FROM payment_orders WHERE user_id=1`)
	require.NoError(t, err)
	id := uuid.NewString()
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := repo.GrantChances(ctx, 42, 1, id, 10, "活动补发"); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	before, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 10, before.ManualChances)
	require.EqualValues(t, 10, before.Chances)
	_, err = repo.GrantChances(ctx, 42, 1, id, 5, "活动补发")
	require.Error(t, err)
	loss := func(_ []service.LuckyDrawPrize, guaranteed bool) (int, error) {
		if guaranteed {
			return 0, nil
		}
		return 1, nil
	}
	request := uuid.NewString()
	batch, err := repo.DrawBatch(ctx, 1, request, c.Version, 10, loss)
	require.NoError(t, err)
	require.Len(t, batch.Records, 10)
	require.Equal(t, 1.5, batch.TotalAwarded)
	for i, r := range batch.Records {
		require.Equal(t, (i+1)%3 == 0, r.Guaranteed)
	}
	after, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, after.Chances)
	require.EqualValues(t, 10, after.DrawsUsed)
	require.Equal(t, 1, after.Misses)
	retry, err := repo.DrawBatch(ctx, 1, request, c.Version-1, 10, loss)
	require.NoError(t, err)
	require.Equal(t, batch, retry)
	_, err = repo.DrawBatch(ctx, 1, request, c.Version, 5, loss)
	require.Error(t, err)
	_, err = repo.DrawBatch(ctx, 1, uuid.NewString(), c.Version, 5, loss)
	require.ErrorIs(t, err, service.ErrLuckyDrawNoChances)
	stats, err := repo.AdminStats(ctx, "1", 1)
	require.NoError(t, err)
	require.EqualValues(t, 10, stats.Summary.ManualChances)
	require.EqualValues(t, 3, stats.Summary.Wins)
	require.Len(t, stats.Grants, 1)
	require.EqualValues(t, 42, stats.Grants[0].AdminID)
	_, err = repo.GrantChances(ctx, 42, 1, uuid.NewString(), 5, "回滚测试")
	require.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE users ADD CONSTRAINT reject_batch_award CHECK (balance<=1.5)`)
	require.NoError(t, err)
	failedRequest := uuid.NewString()
	_, err = repo.DrawBatch(ctx, 1, failedRequest, c.Version, 5, loss)
	require.Error(t, err)
	rolled, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.EqualValues(t, 5, rolled.Chances)
	require.EqualValues(t, 10, rolled.DrawsUsed)
	require.Len(t, rolled.History, 10)
	var stored int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM lucky_draw_batches WHERE request_id=$1`, failedRequest).Scan(&stored))
	require.Zero(t, stored)
}
