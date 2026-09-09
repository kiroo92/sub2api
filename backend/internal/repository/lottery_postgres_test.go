package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// LOTTERY_TEST_DATABASE_URL points at a disposable PostgreSQL database.
// Each test creates and removes its own schema; no application data is used.
func lotteryTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("LOTTERY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set LOTTERY_TEST_DATABASE_URL to run PostgreSQL lottery transaction tests")
	}
	ctx := context.Background()
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("lottery_test_%d", time.Now().UnixNano())
	_, err = base.ExecContext(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(16)
	t.Cleanup(func() {
		_ = db.Close()
		_, dropErr := base.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		require.NoError(t, dropErr)
		_ = base.Close()
	})
	_, err = db.ExecContext(ctx, `CREATE TABLE users (id BIGINT PRIMARY KEY,email TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'active',balance NUMERIC(20,8) NOT NULL DEFAULT 0,total_recharged NUMERIC(20,8) NOT NULL DEFAULT 100,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),deleted_at TIMESTAMPTZ);
 CREATE TABLE api_keys (user_id BIGINT,key TEXT,deleted_at TIMESTAMPTZ);
 CREATE TABLE auth_cache_invalidation_outbox (id BIGSERIAL PRIMARY KEY,cache_key TEXT NOT NULL);`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("238_lottery.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err, "migration is replayable")
	extra, err := migrations.FS.ReadFile("239_lottery_turnstile.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(extra))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO users (id,email) SELECT i, 'person'||i||'@example.com' FROM generate_series(1,65) i;
 INSERT INTO api_keys (user_id,key) SELECT id,'lottery-test-'||id FROM users;`)
	require.NoError(t, err)
	return db
}

func TestLotteryPostgresConcurrentDrawAndRetry(t *testing.T) {
	db := lotteryTestDB(t)
	ctx := context.Background()
	repo := NewLotteryRepository(db)
	svc := service.NewLotteryService(repo, nil, nil)
	before, err := svc.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.False(t, before.Config.Enabled)
	require.Nil(t, before.Current)
	config := service.LotteryConfig{Enabled: true, PrizeAmount: 5, WinnerCount: 6, ParticipantTarget: 60, MinRecharge: 50}
	require.NoError(t, svc.Configure(ctx, config))
	state, err := svc.Snapshot(ctx, 1)
	require.NoError(t, err)
	roundID := state.Current.ID
	_, err = db.ExecContext(ctx, `UPDATE users SET total_recharged=49 WHERE id=65`)
	require.NoError(t, err)
	_, err = svc.Join(ctx, 65, roundID)
	require.ErrorIs(t, err, service.ErrLotteryNotEligible)
	errors := make(chan error, 120)
	var wg sync.WaitGroup
	// Two simultaneous requests per participant, including retries after the draw.
	for id := int64(1); id <= 60; id++ {
		for retry := 0; retry < 2; retry++ {
			wg.Add(1)
			go func(id int64) { defer wg.Done(); _, err := svc.Join(ctx, id, roundID); errors <- err }(id)
		}
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	var entries, winners, outbox int
	var balance, total float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE prize_amount>0) FROM lottery_entries WHERE round_id=$1`, roundID).Scan(&entries, &winners))
	require.Equal(t, 60, entries)
	require.Equal(t, 6, winners)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT SUM(balance),SUM(total_recharged) FROM users WHERE id<=60`).Scan(&balance, &total))
	require.Equal(t, 30.0, balance)
	require.Equal(t, 6000.0, total)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_cache_invalidation_outbox`).Scan(&outbox))
	require.Equal(t, 6, outbox)
	state, err = svc.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.NotEqual(t, roundID, state.Current.ID)
	require.Zero(t, state.Current.ParticipantCount)
	require.Len(t, state.RecentWinners, 6)
	require.Contains(t, state.RecentWinners[0].UserLabel, "***")
	require.Empty(t, state.RecentWinners[0].Email)
	result, err := svc.Join(ctx, 1, roundID)
	require.NoError(t, err)
	require.True(t, result.AlreadyJoined)
	require.True(t, result.Drawn)
	_, err = svc.Join(ctx, 61, roundID)
	require.ErrorIs(t, err, service.ErrLotteryRoundChanged)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT SUM(balance) FROM users`).Scan(&balance))
	require.Equal(t, 30.0, balance)
	// Next-round rules are saved, but do not alter an already opened round.
	config.PrizeAmount = 0.25
	config.ParticipantTarget = 2
	config.WinnerCount = 1
	config.MinRecharge = 0
	config.Enabled = false
	require.NoError(t, svc.Configure(ctx, config))
	_, err = svc.Join(ctx, 1, state.Current.ID)
	require.ErrorIs(t, err, service.ErrLotteryPaused)
	config.Enabled = true
	require.NoError(t, svc.Configure(ctx, config))
	next, err := svc.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 60, next.Current.ParticipantTarget)
	require.Equal(t, 5.0, next.Current.PrizeAmount)
	for id := int64(1); id <= 60; id++ {
		_, err = svc.Join(ctx, id, next.Current.ID)
		require.NoError(t, err)
	}
	next, err = svc.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 2, next.Current.ParticipantTarget)
	require.Equal(t, 0.25, next.Current.PrizeAmount)
	for _, uid := range []int64{1, 2} {
		_, err = svc.Join(ctx, uid, next.Current.ID)
		require.NoError(t, err)
	}
	require.NoError(t, db.QueryRowContext(ctx, `SELECT SUM(balance) FROM users`).Scan(&balance))
	require.Equal(t, 60.25, balance)

}

func TestLotteryPostgresPayoutFailureRollsBack(t *testing.T) {
	db := lotteryTestDB(t)
	ctx := context.Background()
	repo := NewLotteryRepository(db)
	require.NoError(t, repo.Configure(ctx, service.LotteryConfig{Enabled: true, PrizeAmount: 5, WinnerCount: 2, ParticipantTarget: 2, MinRecharge: 0}))
	state, err := repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	id := state.Current.ID
	pick := func(ids []int64, count int) ([]int64, error) { return ids[:count], nil }
	_, err = repo.Join(ctx, 1, id, pick)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE FUNCTION fail_second_credit() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN IF NEW.id=2 THEN RAISE EXCEPTION 'injected credit failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER fail_credit BEFORE UPDATE OF balance ON users FOR EACH ROW EXECUTE FUNCTION fail_second_credit();`)
	require.NoError(t, err)
	_, err = repo.Join(ctx, 2, id, pick)
	require.ErrorContains(t, err, "injected credit failure")
	var balance float64
	var entries, awards, outbox int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT SUM(balance) FROM users`).Scan(&balance))
	require.Zero(t, balance)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE prize_amount>0) FROM lottery_entries`).Scan(&entries, &awards))
	require.Equal(t, 1, entries)
	require.Zero(t, awards)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_cache_invalidation_outbox`).Scan(&outbox))
	require.Zero(t, outbox)
	state, err = repo.Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, id, state.Current.ID)
	require.Equal(t, 1, state.Current.ParticipantCount)
	_, err = db.ExecContext(ctx, `DROP TRIGGER fail_credit ON users`)
	require.NoError(t, err)
	result, err := repo.Join(ctx, 2, id, pick)
	require.NoError(t, err)
	require.True(t, result.Drawn)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT SUM(balance) FROM users`).Scan(&balance))
	require.Equal(t, 10.0, balance)
}

func TestLotteryPostgresTurnstileKeys(t *testing.T) {
	db := lotteryTestDB(t)
	repo := NewLotteryRepository(db)
	ctx := context.Background()
	c := service.LotteryConfig{PrizeAmount: 5, WinnerCount: 6, ParticipantTarget: 60, MinRecharge: 50, TurnstileSiteKey: "site-one", TurnstileSecretKey: "secret-one"}
	require.NoError(t, repo.Configure(ctx, c))
	c.TurnstileSecretKey = ""
	c.PrizeAmount = 6
	require.NoError(t, repo.Configure(ctx, c))
	snapshot, err := repo.Snapshot(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "secret-one", snapshot.Config.TurnstileSecretKey)
	c.TurnstileSiteKey = "site-two"
	c.TurnstileSecretKey = "secret-two"
	require.NoError(t, repo.Configure(ctx, c))
	snapshot, err = repo.Snapshot(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "secret-two", snapshot.Config.TurnstileSecretKey)
	require.Equal(t, "site-two", snapshot.Config.TurnstileSiteKey)
	public, err := service.NewLotteryService(repo, nil, nil).Snapshot(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, public.Config.TurnstileSecretKey)
	require.True(t, public.Config.TurnstileSecretConfigured)
}
