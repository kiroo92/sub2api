package repository

import (
	"context"
	"database/sql"
	"sync"
	"testing"

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
	repo := NewLuckyDrawRepository(db)
	snapshot, err := repo.Snapshot(context.Background(), 1)
	require.NoError(t, err)
	require.False(t, snapshot.Config.Enabled)
	require.Len(t, snapshot.Config.Prizes, 5)
	c := snapshot.Config
	c.Enabled = true
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
	require.NoError(t, repo.Configure(ctx, c))
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
	_, err := db.Exec(`UPDATE users SET total_recharged=149.99 WHERE id=1`)
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
	_, err = db.Exec(`UPDATE users SET total_recharged=150 WHERE id=1`)
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
	_, err := db.Exec(`UPDATE users SET total_recharged=50 WHERE id=1; ALTER TABLE users ADD CONSTRAINT reject_lucky_award CHECK (balance=0)`)
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
