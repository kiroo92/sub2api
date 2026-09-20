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
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// MULTI_SUBSCRIPTION_TEST_DATABASE_URL must point at a disposable test database.
func TestMultiSubscriptionPostgresMigrationAndConcurrentOrder(t *testing.T) {
	dsn := os.Getenv("MULTI_SUBSCRIPTION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set MULTI_SUBSCRIPTION_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("multi_subscription_test_%d", time.Now().UnixNano())
	_, err = base.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() {
		_ = client.Close()
		_, err := base.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
		_ = base.Close()
	})
	require.NoError(t, client.Schema.Create(ctx))
	owner := mustCreateAPIKeyRepoUser(t, ctx, client, "multi-postgres@example.com")
	group, err := client.Group.Create().SetName("multi postgres").SetSubscriptionType(service.SubscriptionTypeSubscription).Save(ctx)
	require.NoError(t, err)
	// Simulate the deployed single-subscription schema before applying the new migration.
	_, err = db.ExecContext(ctx, `ALTER TABLE user_subscriptions DROP COLUMN sort_order;
ALTER TABLE api_keys DROP COLUMN routing_mode;
CREATE UNIQUE INDEX user_subscriptions_user_group_unique_active ON user_subscriptions(user_id,group_id) WHERE deleted_at IS NULL;`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at,assigned_at,created_at,updated_at) VALUES ($1,$2,NOW(),NOW()+INTERVAL '30 days',NOW(),NOW(),NOW())`, owner.ID, group.ID)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("240_multi_subscriptions.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	repo, ok := NewUserSubscriptionRepository(client).(*userSubscriptionRepository)
	require.True(t, ok)
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sub := &service.UserSubscription{UserID: owner.ID, GroupID: group.ID, StartsAt: time.Now(), ExpiresAt: time.Now().AddDate(0, 0, 30), Status: service.SubscriptionStatusActive}
			errors <- repo.Create(ctx, sub)
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	rows, err := repo.ListActiveByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, rows, 9)
	ids := make([]int64, len(rows))
	for i := range rows {
		require.Equal(t, i, rows[i].SortOrder)
		ids[len(rows)-1-i] = rows[i].ID
	}
	require.NoError(t, repo.ReorderActive(ctx, owner.ID, ids))
	rows, err = repo.ListActiveByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, ids[0], rows[0].ID)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	rows, err = repo.ListActiveByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, ids[0], rows[0].ID, "replaying migration preserves the saved order")
	_, err = client.APIKey.Create().SetUserID(owner.ID).SetName("conflict").SetKey("test-multi-conflict").SetGroupID(group.ID).SetRoutingMode(service.APIKeyRoutingAllSubscriptions).Save(ctx)
	require.Error(t, err, "database enforces mode exclusivity")
	key, err := client.APIKey.Create().SetUserID(owner.ID).SetName("all").SetKey("test-multi-all").SetRoutingMode(service.APIKeyRoutingAllSubscriptions).Save(ctx)
	require.NoError(t, err)
	for _, filename := range []string{"071_add_usage_billing_dedup.sql", "073_add_usage_billing_dedup_archive.sql"} {
		content, err := migrations.FS.ReadFile(filename)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, string(content))
		require.NoError(t, err)
	}
	admittedID := rows[0].ID
	require.NoError(t, repo.ExtendExpiry(ctx, admittedID, time.Now().Add(-time.Hour)))
	billing := NewUsageBillingRepository(client, db)
	command := &service.UsageBillingCommand{RequestID: "late-subscription-request", APIKeyID: key.ID, UserID: owner.ID, SubscriptionID: &admittedID, SubscriptionCost: 2.5}
	result, err := billing.Apply(ctx, command)
	require.NoError(t, err)
	require.True(t, result.Applied)
	result, err = billing.Apply(ctx, command)
	require.NoError(t, err)
	require.False(t, result.Applied)
	admitted, err := repo.GetByID(ctx, admittedID)
	require.NoError(t, err)
	require.Equal(t, 2.5, admitted.DailyUsageUSD)
	other, err := repo.GetByID(ctx, rows[1].ID)
	require.NoError(t, err)
	require.Zero(t, other.DailyUsageUSD)
	uncharged, err := client.User.Get(ctx, owner.ID)
	require.NoError(t, err)
	require.Zero(t, uncharged.Balance)
	// Late completion after revocation still belongs to the admitted row.
	require.NoError(t, repo.Delete(ctx, admittedID))
	late := &service.UsageBillingCommand{RequestID: "revoked-inflight-request", APIKeyID: key.ID, UserID: owner.ID, SubscriptionID: &admittedID, SubscriptionCost: 1, AdmittedSubscription: true}
	_, err = billing.Apply(ctx, late)
	require.NoError(t, err)
	admitted, err = repo.GetByIDIncludeDeleted(ctx, admittedID)
	require.NoError(t, err)
	require.Equal(t, 3.5, admitted.DailyUsageUSD)
	// Exercise generic paid fulfillment against real repositories, including replay.
	groupRepo := NewGroupRepository(client, db)
	subs := service.NewSubscriptionService(groupRepo, repo, nil, client, nil)
	defer subs.Stop()
	payments := service.NewPaymentService(client, nil, nil, nil, subs, nil, nil, groupRepo, nil)
	for i := 0; i < 2; i++ {
		order, err := client.PaymentOrder.Create().SetUserID(owner.ID).SetUserEmail(owner.Email).SetUserName("test").
			SetAmount(10).SetPayAmount(10).SetFeeRate(0).SetRechargeCode(fmt.Sprintf("sub-paid-%d", i)).SetOutTradeNo(fmt.Sprintf("sub-paid-%d", i)).
			SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo(fmt.Sprintf("trade-%d", i)).SetOrderType(payment.OrderTypeSubscription).SetPlanID(1).SetSubscriptionGroupID(group.ID).SetSubscriptionDays(7).
			SetStatus(service.OrderStatusPaid).SetPaidAt(time.Now()).SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("test").Save(ctx)
		require.NoError(t, err)
		require.NoError(t, payments.ExecuteSubscriptionFulfillment(ctx, order.ID))
		require.NoError(t, payments.ExecuteSubscriptionFulfillment(ctx, order.ID))
	}
	rows, err = repo.ListByUserID(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, rows, 10, "eight original non-revoked rows plus two independent purchases")
	require.NotEqual(t, rows[8].ID, rows[9].ID)
}
