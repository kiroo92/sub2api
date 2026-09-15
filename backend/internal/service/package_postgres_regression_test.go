package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// PACKAGE_TEST_DATABASE_URL must refer to a disposable local test database.
// Each test owns an isolated schema and never touches application tables.
func packageRegressionDatabase(t *testing.T) (*sql.DB, *dbent.Client) {
	t.Helper()
	dsn := os.Getenv("PACKAGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PACKAGE_TEST_DATABASE_URL to run independent-package PostgreSQL regressions")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("package_regression_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(12)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() {
		_ = client.Close()
		_, cleanupErr := base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, cleanupErr)
		_ = base.Close()
	})
	_, err = db.Exec(`
CREATE TABLE users(id BIGINT PRIMARY KEY);
CREATE TABLE groups(id BIGINT PRIMARY KEY);
CREATE TABLE api_keys(id BIGINT PRIMARY KEY,group_id BIGINT);
CREATE TABLE payment_orders(id BIGINT PRIMARY KEY,status TEXT NOT NULL DEFAULT 'unchanged');
CREATE TABLE user_subscriptions(id BIGINT PRIMARY KEY,user_id BIGINT,group_id BIGINT,weekly_usage_usd NUMERIC(20,8),UNIQUE(user_id,group_id));
INSERT INTO users VALUES (1),(2),(3),(4);
INSERT INTO groups VALUES (1);
INSERT INTO payment_orders(id) VALUES (1),(2),(3),(4),(5);
INSERT INTO user_subscriptions VALUES (1,1,1,42.5);`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("240_packages.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	return db, client
}

func TestPackagePostgresIssuanceSettlementAndOrdering(t *testing.T) {
	db, client := packageRegressionDatabase(t)
	ctx := context.Background()
	svc := NewPackageService(client, nil)
	plan := PackagePlan{ID: 1, GroupID: 1, Name: "Monthly test", Price: 385, Currency: "CNY", ValidityDays: 30, BaseQuotaUSD: 1800, GroupBuyEnabled: true, GroupBuyHours: 48, ForSale: true, Tiers: []PackageTier{{Members: 3, QuotaUSD: 1980}, {Members: 5, QuotaUSD: 2100}, {Members: 10, QuotaUSD: 2400}}}
	require.NoError(t, plan.Validate())
	snapshot, err := json.Marshal(plan)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO package_plans(id,group_id,terms,for_sale) VALUES(1,1,$1,true)`, string(snapshot))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO package_group_buys(id,creator_id,plan_id,plan_snapshot) VALUES(1,1,1,$1)`, string(snapshot))
	require.NoError(t, err)
	for id := int64(1); id <= 3; id++ {
		_, err = db.Exec(`INSERT INTO package_orders(order_id,user_id,plan_id,plan_snapshot,group_buy_id) VALUES($1,$1,1,$2,1)`, id, string(snapshot))
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT INTO package_orders(order_id,user_id,plan_id,plan_snapshot,group_buy_id) VALUES(5,4,1,$1,1)`, string(snapshot))
	require.NoError(t, err)
	paidAt := time.Now().UTC().Add(-8 * 24 * time.Hour).Truncate(time.Microsecond)
	order := &dbent.PaymentOrder{ID: 1, UserID: 1, PaidAt: &paidAt}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- svc.fulfill(ctx, order) }()
	}
	wg.Wait()
	close(results)
	for e := range results {
		require.NoError(t, e)
	}
	holdings, err := svc.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Len(t, holdings, 1, "concurrent notifications issue one package")
	first := holdings[0]
	require.Len(t, first.Periods, 4)
	require.NotNil(t, first.CurrentPeriod)
	require.Equal(t, 2, first.CurrentPeriod.Index, "payment timestamp, not fulfillment timestamp, determines period")
	require.Equal(t, 450.0, first.CurrentPeriod.QuotaUSD)
	require.Equal(t, 9*24*time.Hour, first.Periods[3].EndsAt.Sub(first.Periods[3].StartsAt))
	require.True(t, first.StartsAt.Equal(paidAt))
	_, err = db.Exec(`UPDATE package_periods SET used_usd=23 WHERE id=$1`, first.Periods[0].ID)
	require.NoError(t, err)
	for id := int64(2); id <= 3; id++ {
		require.NoError(t, svc.fulfill(ctx, &dbent.PaymentOrder{ID: id, UserID: id, PaidAt: &paidAt}))
	}
	// Changing current sale terms cannot change a running group's paid snapshot.
	_, err = db.Exec(`UPDATE package_plans SET terms=jsonb_set(terms,'{base_quota_usd}','9000') WHERE id=1`)
	require.NoError(t, err)
	// No payment service/provider is configured: settlement needs only the
	// delivered memberships, regardless of payment channel availability.
	require.NoError(t, svc.SettleExpiredGroups(ctx))
	require.NoError(t, svc.SettleExpiredGroups(ctx))
	group, err := svc.GetGroup(ctx, 1, 1)
	require.NoError(t, err)
	require.Equal(t, "settled", group.Status)
	require.NotNil(t, group.FinalMembers)
	require.Equal(t, 3, *group.FinalMembers)
	require.NotNil(t, group.FinalQuotaUSD)
	require.Equal(t, 1980.0, *group.FinalQuotaUSD)
	var unchangedOrders int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM payment_orders WHERE status='unchanged'`).Scan(&unchangedOrders))
	require.Equal(t, 5, unchangedOrders, "group reward settlement never changes payment order states")
	holdings, err = svc.ListOwned(ctx, 1)
	require.NoError(t, err)
	for _, period := range holdings[0].Periods {
		require.Equal(t, 495.0, period.QuotaUSD)
	}
	require.Equal(t, 23.0, holdings[0].Periods[0].UsedUSD)
	require.Equal(t, 0.0, holdings[0].CurrentPeriod.UsedUSD, "past-period use does not move into the current allocation")
	_, err = db.Exec(`INSERT INTO package_orders(order_id,user_id,plan_id,plan_snapshot) VALUES(4,1,1,$1)`, string(snapshot))
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, svc.fulfill(ctx, &dbent.PaymentOrder{ID: 4, UserID: 1, PaidAt: &now}))
	holdings, err = svc.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Len(t, holdings, 2, "same group can contain independent packages")
	secondID := holdings[1].ID
	require.NoError(t, svc.Reorder(ctx, 1, []int64{secondID, first.ID}))
	require.Error(t, svc.Reorder(ctx, 1, []int64{secondID, secondID}))
	holdings, err = svc.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, secondID, holdings[0].ID)
	var legacyCount int
	var legacyUsage float64
	require.NoError(t, db.QueryRow(`SELECT COUNT(*),MAX(weekly_usage_usd) FROM user_subscriptions`).Scan(&legacyCount, &legacyUsage))
	require.Equal(t, 1, legacyCount)
	require.Equal(t, 42.5, legacyUsage, "new issuance and settlement leave old subscription data unchanged")
}
