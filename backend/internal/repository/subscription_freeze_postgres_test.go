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

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionFreezePostgres(t *testing.T) {
	dsn := os.Getenv("SUBSCRIPTION_FREEZE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SUBSCRIPTION_FREEZE_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("freeze_test_%d", time.Now().UnixNano())
	_, err = base.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		_, e := base.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, e)
		require.NoError(t, base.Close())
	})
	require.NoError(t, client.Schema.Create(ctx))
	_, err = db.ExecContext(ctx, `ALTER TABLE user_subscriptions DROP COLUMN frozen_at, DROP COLUMN frozen_duration_us, DROP COLUMN admin_assignment_key, DROP COLUMN admin_assignment_fingerprint`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("244_subscription_freeze.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = db.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	owner := mustCreateAPIKeyRepoUser(t, ctx, client, "freeze-owner@example.com")
	group, err := client.Group.Create().SetName("freeze group").SetSubscriptionType(service.SubscriptionTypeSubscription).Save(ctx)
	require.NoError(t, err)
	repo := NewUserSubscriptionRepository(client).(*userSubscriptionRepository)
	svc := service.NewSubscriptionService(NewGroupRepository(client, db), repo, nil, client, nil)
	t.Cleanup(svc.Stop)
	input := &service.AssignSubscriptionInput{UserID: owner.ID, GroupID: group.ID, AssignedBy: owner.ID, ValidityDays: 30, OperationKey: "assign:first"}
	first, err := svc.AssignSubscription(ctx, input)
	require.NoError(t, err)
	replay, err := svc.AssignSubscription(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first.ID, replay.ID)
	input.OperationKey = "assign:second"
	second, err := svc.AssignSubscription(ctx, input)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	_, err = svc.SetUserSubscriptionFrozen(ctx, owner.ID, first.ID, true)
	require.ErrorIs(t, err, service.ErrSubscriptionFreezeDisabled)
	_, err = client.Setting.Create().SetKey(service.SettingKeySubscriptionFreezeEnabled).SetValue("true").Save(ctx)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	daily := timezone.StartOfDay(now)
	periodic := now.Add(-24 * time.Hour)
	_, err = client.UserSubscription.UpdateOneID(first.ID).SetDailyWindowStart(daily).SetWeeklyWindowStart(periodic).SetMonthlyWindowStart(periodic).SetDailyUsageUsd(3).SetWeeklyUsageUsd(5).SetMonthlyUsageUsd(7).Save(ctx)
	require.NoError(t, err)
	frozen, err := svc.SetUserSubscriptionFrozen(ctx, owner.ID, first.ID, true)
	require.NoError(t, err)
	_, err = svc.SetUserSubscriptionFrozen(ctx, owner.ID+999, first.ID, false)
	require.Error(t, err)
	again, err := svc.SetUserSubscriptionFrozen(ctx, owner.ID, first.ID, true)
	require.NoError(t, err)
	require.True(t, frozen.FrozenAt.Equal(*again.FrozenAt))
	active, err := svc.GetActiveSubscription(ctx, owner.ID, group.ID)
	require.NoError(t, err)
	require.Equal(t, second.ID, active.ID)
	require.NoError(t, repo.ReorderActive(ctx, owner.ID, []int64{second.ID}))
	require.NoError(t, repo.ReorderActive(ctx, owner.ID, []int64{second.ID, first.ID}))
	require.NoError(t, repo.ResetDailyUsage(ctx, first.ID, frozen.DailyWindowStart, time.Now().Add(time.Hour)))
	require.NoError(t, repo.IncrementUsage(ctx, first.ID, 2)) // late pre-freeze usage must still be recorded
	// Move the whole saved paused timeline into the past to simulate a long holiday.
	shift := -40 * 24 * time.Hour
	_, err = client.UserSubscription.UpdateOneID(first.ID).SetFrozenAt(frozen.FrozenAt.Add(shift)).SetExpiresAt(frozen.ExpiresAt.Add(shift)).
		SetDailyWindowStart(frozen.DailyWindowStart.Add(shift)).SetWeeklyWindowStart(frozen.WeeklyWindowStart.Add(shift)).SetMonthlyWindowStart(frozen.MonthlyWindowStart.Add(shift)).Save(ctx)
	require.NoError(t, err)
	_, err = repo.BatchUpdateExpiredStatus(ctx)
	require.NoError(t, err)
	frozen, err = repo.GetByID(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusActive, frozen.Status)
	left := frozen.ExpiresAt.Sub(*frozen.FrozenAt)
	dailyLeft := frozen.DailyResetTime().Sub(*frozen.FrozenAt)
	_, err = client.Setting.Update().Where(setting.KeyEQ(service.SettingKeySubscriptionFreezeEnabled)).SetValue("false").Save(ctx)
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := svc.SetUserSubscriptionFrozen(ctx, owner.ID, first.ID, false)
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	for e := range results {
		require.NoError(t, e)
	}
	thawed, err := repo.GetByID(ctx, first.ID)
	require.NoError(t, err)
	require.Nil(t, thawed.FrozenAt)
	require.InDelta(t, left.Seconds(), time.Until(thawed.ExpiresAt).Seconds(), 2)
	require.InDelta(t, dailyLeft.Seconds(), time.Until(*thawed.DailyResetTime()).Seconds(), 2)
	require.Equal(t, 5.0, thawed.DailyUsageUSD)
	require.Equal(t, 7.0, thawed.WeeklyUsageUSD)
	require.Equal(t, 9.0, thawed.MonthlyUsageUSD)
	expiry := thawed.ExpiresAt
	thawed, err = svc.SetUserSubscriptionFrozen(ctx, owner.ID, first.ID, false)
	require.NoError(t, err)
	require.True(t, expiry.Equal(thawed.ExpiresAt))
	rows, err := repo.ListByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, second.ID, rows[0].ID)
	_, err = client.Setting.Update().Where(setting.KeyEQ(service.SettingKeySubscriptionFreezeEnabled)).SetValue("true").Save(ctx)
	require.NoError(t, err)
	_, err = svc.SetUserSubscriptionFrozen(ctx, owner.ID, first.ID, true)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatus(ctx, first.ID, service.SubscriptionStatusSuspended))
	thawed, err = svc.SetUserSubscriptionFrozen(ctx, owner.ID, first.ID, false)
	require.NoError(t, err)
	require.Equal(t, service.SubscriptionStatusSuspended, thawed.Status)
	require.NoError(t, repo.Delete(ctx, first.ID))
	_, err = svc.SetUserSubscriptionFrozen(ctx, owner.ID, first.ID, false)
	require.Error(t, err)
	// Replay after revocation cannot grant a replacement row.
	input.OperationKey = "assign:first"
	replay, err = svc.AssignSubscription(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first.ID, replay.ID)
	require.NotNil(t, replay.DeletedAt)
	input.OperationKey = "assign:second"
	input.ValidityDays = 14
	_, err = svc.AssignSubscription(ctx, input)
	require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
	batch := &service.BulkAssignSubscriptionInput{UserIDs: []int64{owner.ID, owner.ID}, GroupID: group.ID, AssignedBy: owner.ID, ValidityDays: 30, OperationKey: "batch:new"}
	b1, err := svc.BulkAssignSubscription(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, 1, b1.SuccessCount)
	b2, err := svc.BulkAssignSubscription(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, b1.Subscriptions[0].ID, b2.Subscriptions[0].ID)
	// Frozen administrative adjustments and quota resets preserve the paused state.
	third := b1.Subscriptions[0]
	paused, err := svc.SetUserSubscriptionFrozen(ctx, owner.ID, third.ID, true)
	require.NoError(t, err)
	pausedExpiry := paused.ExpiresAt
	extended, err := svc.ExtendSubscription(ctx, third.ID, 2)
	require.NoError(t, err)
	require.NotNil(t, extended.FrozenAt)
	require.Equal(t, pausedExpiry.AddDate(0, 0, 2), extended.ExpiresAt)
	require.NoError(t, repo.IncrementUsage(ctx, third.ID, 3))
	reset, err := svc.AdminResetQuota(ctx, third.ID, true, true, true)
	require.NoError(t, err)
	require.NotNil(t, reset.FrozenAt)
	require.Zero(t, reset.DailyUsageUSD)
	require.False(t, reset.WeeklyWindowStart.After(*reset.FrozenAt))
	// A second intentional allocation to this owner/group does not touch that row.
	input.OperationKey = "assign:after-freeze"
	input.ValidityDays = 30
	fresh, err := svc.AssignSubscription(ctx, input)
	require.NoError(t, err)
	require.NotEqual(t, third.ID, fresh.ID)
	require.Nil(t, fresh.FrozenAt)
	kept, err := repo.GetByID(ctx, third.ID)
	require.NoError(t, err)
	require.NotNil(t, kept.FrozenAt)
	// Batch partial success then retry uses the durable per-user marker.
	batch.OperationKey = "batch:partial"
	batch.UserIDs = []int64{owner.ID, owner.ID + 1000000}
	partial, err := svc.BulkAssignSubscription(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, 1, partial.SuccessCount)
	require.Equal(t, 1, partial.FailedCount)
	retry, err := svc.BulkAssignSubscription(ctx, batch)
	require.NoError(t, err)
	require.Equal(t, partial.Subscriptions[0].ID, retry.Subscriptions[0].ID)
	restored, err := svc.RestoreSubscription(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, first.ID, restored.ID)
	require.Equal(t, service.SubscriptionStatusSuspended, restored.Status, "admin restoration does not remove the original suspension")
}
