package service

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
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

type packageTestGroups struct{ GroupRepository }

func (packageTestGroups) GetByID(context.Context, int64) (*Group, error) {
	return &Group{ID: 1, Name: "Test", Platform: "openai", Status: StatusActive}, nil
}

// PACKAGE_TEST_DATABASE_URL must address a disposable test database. Each run
// confines all writes and cleanup to a new uniquely named schema.
func packageTestService(t *testing.T) (*PackageService, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("PACKAGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PACKAGE_TEST_DATABASE_URL to exercise PostgreSQL transactions")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("package_test_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
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
		_, e := base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, e)
		_ = base.Close()
	})
	_, err = db.Exec(`CREATE TABLE users(id BIGINT PRIMARY KEY);CREATE TABLE groups(id BIGINT PRIMARY KEY);CREATE TABLE api_keys(id BIGINT PRIMARY KEY,group_id BIGINT);INSERT INTO users SELECT generate_series(1,25);INSERT INTO groups VALUES(1);`)
	require.NoError(t, err)
	for _, name := range []string{"092_payment_orders.sql", "102_add_out_trade_no_to_payment_orders.sql"} {
		data, e := migrations.FS.ReadFile(name)
		require.NoError(t, e)
		_, e = db.Exec(string(data))
		require.NoError(t, e)
	}
	_, err = db.Exec(`ALTER TABLE payment_orders ADD COLUMN provider_key VARCHAR(30);ALTER TABLE payment_orders ADD COLUMN provider_snapshot JSONB;`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("240_packages.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	return NewPackageService(client, packageTestGroups{}), db
}

func packageTestOrder(ctx context.Context, s *PackageService, p *PackagePlan, oid, uid, gid int64, paid time.Time) (*dbent.PaymentOrder, error) {
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	req := CreateOrderRequest{UserID: uid, OrderType: OrderTypePackage, PackagePlanID: p.ID, GroupBuyID: gid, packagePlan: p}
	if err = s.lockCheckout(ctx, tx.Client(), req); err != nil {
		return nil, err
	}
	if _, err = tx.Client().ExecContext(ctx, `INSERT INTO payment_orders(id,user_id,amount,pay_amount,expires_at,out_trade_no,order_type) VALUES($1,$2,$3,$3,NOW()+INTERVAL '30 minutes',$4,'package')`, oid, uid, p.Price, fmt.Sprintf("package-test-%d", oid)); err != nil {
		return nil, err
	}
	if err = s.attachOrder(ctx, tx.Client(), req, oid); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &dbent.PaymentOrder{ID: oid, UserID: uid, OrderType: OrderTypePackage, PaidAt: &paid}, nil
}

func TestPackagePostgresFulfillSettleAndReorder(t *testing.T) {
	s, db := packageTestService(t)
	ctx := context.Background()
	plan, err := s.SavePlan(ctx, testPackagePlan())
	require.NoError(t, err)
	group, err := s.CreateGroup(ctx, 1, plan.ID)
	require.NoError(t, err)
	require.Equal(t, "draft", group.Status)
	_, err = s.GetGroup(ctx, 2, group.ID)
	require.Error(t, err, "draft must remain private")
	now := time.Now().UTC().Truncate(time.Microsecond)
	first, err := packageTestOrder(ctx, s, plan, 1, 1, group.ID, now)
	require.NoError(t, err)
	require.NoError(t, s.fulfill(ctx, first))
	require.NoError(t, s.fulfill(ctx, first))
	owned, err := s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Len(t, owned, 1)
	require.Equal(t, 450.0, owned[0].CurrentPeriod.QuotaUSD)
	firstID := owned[0].ID
	periodID := owned[0].CurrentPeriod.ID
	_, err = db.Exec(`UPDATE package_periods SET used_usd=50 WHERE id=$1`, periodID)
	require.NoError(t, err)
	group, err = s.GetGroup(ctx, 1, group.ID)
	require.NoError(t, err)
	require.Equal(t, "open", group.Status)
	require.Equal(t, 1, group.PaidCount)
	require.True(t, group.Joined)
	// Reserve more than the last available seat before paying. Late verified
	// payments receive the frozen award, without rewriting the final count.
	orders := []*dbent.PaymentOrder{}
	for uid := int64(2); uid <= 11; uid++ {
		o, e := packageTestOrder(ctx, s, plan, uid, uid, group.ID, now)
		require.NoError(t, e)
		orders = append(orders, o)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 18)
	for _, o := range orders[:9] {
		for retry := 0; retry < 2; retry++ {
			wg.Add(1)
			go func(o *dbent.PaymentOrder) { defer wg.Done(); errs <- s.fulfill(ctx, o) }(o)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	group, err = s.GetGroup(ctx, 1, group.ID)
	require.NoError(t, err)
	require.Equal(t, "settled", group.Status)
	require.Equal(t, 10, *group.FinalMembers)
	require.Equal(t, 2400.0, *group.FinalQuotaUSD)
	require.NoError(t, s.fulfill(ctx, orders[9]))
	group, err = s.GetGroup(ctx, 11, group.ID)
	require.NoError(t, err)
	require.Equal(t, 10, *group.FinalMembers)
	require.True(t, group.Joined)
	owned, err = s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 600.0, owned[0].CurrentPeriod.QuotaUSD)
	require.Equal(t, 50.0, owned[0].CurrentPeriod.UsedUSD)
	require.Equal(t, now, owned[0].StartsAt)
	require.Equal(t, now.Add(30*24*time.Hour), owned[0].ExpiresAt)
	second, err := packageTestOrder(ctx, s, plan, 20, 1, 0, now.Add(time.Second))
	require.NoError(t, err)
	require.NoError(t, s.fulfill(ctx, second))
	owned, err = s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Len(t, owned, 2)
	secondID := owned[1].ID
	require.NoError(t, s.Reorder(ctx, 1, []int64{secondID, firstID}))
	owned, err = s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, secondID, owned[0].ID)
	require.Error(t, s.Reorder(ctx, 1, []int64{firstID, firstID}))
	require.Error(t, s.Reorder(ctx, 1, []int64{firstID, 999}))
	third, err := packageTestOrder(ctx, s, plan, 21, 1, 0, now.Add(2*time.Second))
	require.NoError(t, err)
	require.NoError(t, s.fulfill(ctx, third))
	owned, err = s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Len(t, owned, 3)
	require.Equal(t, secondID, owned[0].ID)
	require.Equal(t, int64(21), owned[2].OrderID)
}

func TestPackagePostgresUniqueCheckoutAndPartialTier(t *testing.T) {
	s, db := packageTestService(t)
	ctx := context.Background()
	p, err := s.SavePlan(ctx, testPackagePlan())
	require.NoError(t, err)
	g, err := s.CreateGroup(ctx, 1, p.ID)
	require.NoError(t, err)
	first, err := packageTestOrder(ctx, s, p, 1, 1, g.ID, time.Now())
	require.NoError(t, err)
	require.NoError(t, s.fulfill(ctx, first))
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for oid := int64(2); oid <= 3; oid++ {
		wg.Add(1)
		go func(oid int64) {
			defer wg.Done()
			_, e := packageTestOrder(ctx, s, p, oid, 2, g.ID, time.Now())
			results <- e
		}(oid)
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	require.Equal(t, 1, success)
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM package_orders WHERE group_buy_id=$1 AND user_id=2`, g.ID).Scan(&count))
	require.Equal(t, 1, count)
	// An unpaid reservation does not affect settlement; 1 paid settles at base.
	tx, err := s.client.Tx(ctx)
	require.NoError(t, err)
	locked, err := packageLockedGroup(ctx, tx.Client(), g.ID)
	require.NoError(t, err)
	require.NoError(t, settlePackageGroup(ctx, tx.Client(), locked))
	require.NoError(t, tx.Commit())
	g, err = s.GetGroup(ctx, 1, g.ID)
	require.NoError(t, err)
	require.Equal(t, 1, *g.FinalMembers)
	require.Equal(t, 1800.0, *g.FinalQuotaUSD)
	var reservedID int64
	require.NoError(t, db.QueryRow(`SELECT order_id FROM package_orders WHERE group_buy_id=$1 AND user_id=2`, g.ID).Scan(&reservedID))
	paid := time.Now()
	require.NoError(t, s.fulfill(ctx, &dbent.PaymentOrder{ID: reservedID, UserID: 2, PaidAt: &paid}))
	owned, err := s.ListOwned(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, 450.0, owned[0].CurrentPeriod.QuotaUSD)
	_, err = packageTestOrder(ctx, s, p, 4, 3, g.ID, time.Now())
	require.Error(t, err)
	require.NoError(t, s.StopSale(ctx, p.ID))
	_, err = s.checkoutPlan(ctx, CreateOrderRequest{UserID: 3, PackagePlanID: p.ID})
	require.Error(t, err)
	// Existing paid holdings and catalog snapshots survive sale/config changes.
	owned, err = s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1800.0, owned[0].Plan.BaseQuotaUSD)
}

func TestPackagePostgresPeriodBoundaryAndDeadline(t *testing.T) {
	s, db := packageTestService(t)
	ctx := context.Background()
	plan, err := s.SavePlan(ctx, testPackagePlan())
	require.NoError(t, err)
	started := time.Now().Add(-28 * 24 * time.Hour)
	o, err := packageTestOrder(ctx, s, plan, 1, 1, 0, started)
	require.NoError(t, err)
	require.NoError(t, s.fulfill(ctx, o))
	owned, err := s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Len(t, owned[0].Periods, 4)
	require.Equal(t, 4, owned[0].CurrentPeriod.Index)
	_, err = db.Exec(`UPDATE package_periods SET used_usd=500 WHERE package_id=$1 AND period_index=1`, owned[0].ID)
	require.NoError(t, err)
	owned, err = s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 0.0, owned[0].CurrentPeriod.UsedUSD)
	require.Equal(t, 500.0, owned[0].Periods[0].UsedUSD)
	g, err := s.CreateGroup(ctx, 2, plan.ID)
	require.NoError(t, err)
	first, err := packageTestOrder(ctx, s, plan, 2, 2, g.ID, time.Now())
	require.NoError(t, err)
	require.NoError(t, s.fulfill(ctx, first))
	pending, err := packageTestOrder(ctx, s, plan, 3, 3, g.ID, time.Now())
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE package_group_buys SET ends_at=NOW()-INTERVAL '1 hour' WHERE id=$1`, g.ID)
	require.NoError(t, err)
	require.NoError(t, s.fulfill(ctx, pending))
	g, err = s.GetGroup(ctx, 2, g.ID)
	require.NoError(t, err)
	require.Equal(t, 1, g.PaidCount, "payment after deadline receives entitlement but does not alter the deadline tier")
	late, err := s.ListOwned(ctx, 3)
	require.NoError(t, err)
	require.Len(t, late, 1)
	require.Equal(t, 450.0, late[0].CurrentPeriod.QuotaUSD)
	plan.BaseQuotaUSD = 1600
	_, err = s.SavePlan(ctx, *plan)
	require.NoError(t, err)
	g, err = s.GetGroup(ctx, 2, g.ID)
	require.NoError(t, err)
	require.Equal(t, 1800.0, g.Plan.BaseQuotaUSD, "group terms remain frozen")
}

func TestPackagePostgresGenericPaidDispatchDeliversBaseOnce(t *testing.T) {
	s, db := packageTestService(t)
	ctx := context.Background()
	auditSchema, err := migrations.FS.ReadFile("093_payment_audit_logs.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(auditSchema))
	require.NoError(t, err)
	plan, err := s.SavePlan(ctx, testPackagePlan())
	require.NoError(t, err)
	group, err := s.CreateGroup(ctx, 1, plan.ID)
	require.NoError(t, err)
	_, err = packageTestOrder(ctx, s, plan, 1, 1, group.ID, time.Now())
	require.NoError(t, err)
	order, err := s.client.PaymentOrder.Get(ctx, 1)
	require.NoError(t, err)
	pay := &PaymentService{entClient: s.client, packageSvc: s}
	require.Error(t, pay.executePackageFulfillment(ctx, order), "an unpaid preparation never issues an entitlement")
	owned, err := s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, owned)
	// This is the existing post-verification dispatcher used by every product.
	require.NoError(t, pay.toPaid(ctx, order, "verified-trade", plan.Price, "alipay"))
	require.NoError(t, pay.toPaid(ctx, order, "verified-trade", plan.Price, "alipay"))
	current, err := s.client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, current.Status)
	owned, err = s.ListOwned(ctx, 1)
	require.NoError(t, err)
	require.Len(t, owned, 1)
	require.Equal(t, 450.0, owned[0].CurrentPeriod.QuotaUSD)
	require.True(t, owned[0].StartsAt.Equal(*current.PaidAt))
	group, err = s.GetGroup(ctx, 1, group.ID)
	require.NoError(t, err)
	require.Equal(t, 1, group.PaidCount)
	require.Equal(t, "open", group.Status, "base delivery does not wait for group completion")
}

func TestPackagePostgresMultipleOpenGroupsDeliverBaseAndSettleIndependently(t *testing.T) {
	s, db := packageTestService(t)
	ctx := context.Background()
	auditSchema, err := migrations.FS.ReadFile("093_payment_audit_logs.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(auditSchema))
	require.NoError(t, err)
	plan, err := s.SavePlan(ctx, testPackagePlan())
	require.NoError(t, err)
	pay := &PaymentService{entClient: s.client, packageSvc: s}
	buy := func(oid, uid, gid int64) {
		t.Helper()
		_, e := packageTestOrder(ctx, s, plan, oid, uid, gid, time.Now())
		require.NoError(t, e)
		order, e := s.client.PaymentOrder.Get(ctx, oid)
		require.NoError(t, e)
		require.NoError(t, pay.toPaid(ctx, order, fmt.Sprintf("verified-%d", oid), plan.Price, "alipay"))
	}
	var groups []int64
	for i := 0; i < 3; i++ {
		group, e := s.CreateGroup(ctx, 1, plan.ID)
		require.NoError(t, e)
		require.NotContains(t, groups, group.ID, "an earlier open group must not capture a new purchase")
		groups = append(groups, group.ID)
		buy(int64(i+1), 1, group.ID)
		owned, e := s.ListOwned(ctx, 1)
		require.NoError(t, e)
		require.Len(t, owned, i+1)
		for _, holding := range owned {
			require.NotNil(t, holding.CurrentPeriod, "each purchase is immediately active")
			require.Equal(t, 450.0, holding.CurrentPeriod.QuotaUSD)
			order, e := s.client.PaymentOrder.Get(ctx, holding.OrderID)
			require.NoError(t, e)
			require.Equal(t, OrderStatusCompleted, order.Status)
			require.True(t, holding.StartsAt.Equal(*order.PaidAt))
		}
	}
	// The same participant can also join every distinct group.
	for i, gid := range groups {
		buy(int64(10+i), 2, gid)
	}
	buy(20, 3, groups[0])
	_, err = packageTestOrder(ctx, s, plan, 21, 1, groups[0], time.Now())
	require.Error(t, err, "same-group uniqueness remains intact")
	for _, gid := range groups {
		group, e := s.GetGroup(ctx, 1, gid)
		require.NoError(t, e)
		require.Equal(t, "open", group.Status)
	}
	_, err = db.Exec(`UPDATE package_periods SET used_usd=23 WHERE package_id IN(SELECT id FROM user_packages WHERE user_id=1 AND group_buy_id=$1) AND period_index=1`, groups[0])
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE package_group_buys SET ends_at=NOW()-INTERVAL '1 second' WHERE id=$1`, groups[0])
	require.NoError(t, err)
	require.NoError(t, s.SettleExpiredGroups(ctx))
	require.NoError(t, s.SettleExpiredGroups(ctx))
	for i, gid := range groups {
		group, e := s.GetGroup(ctx, 1, gid)
		require.NoError(t, e)
		if i == 0 {
			require.Equal(t, "settled", group.Status)
			require.Equal(t, 3, *group.FinalMembers)
			require.Equal(t, 1980.0, *group.FinalQuotaUSD)
		} else {
			require.Equal(t, "open", group.Status)
			require.Nil(t, group.FinalQuotaUSD)
		}
	}
	for _, uid := range []int64{1, 2} {
		owned, e := s.ListOwned(ctx, uid)
		require.NoError(t, e)
		require.Len(t, owned, 3)
		for i, holding := range owned {
			quota := 450.0
			if i == 0 {
				quota = 495
			}
			require.NotNil(t, holding.CurrentPeriod)
			for _, period := range holding.Periods {
				require.Equal(t, quota, period.QuotaUSD)
			}
		}
		if uid == 1 {
			require.Equal(t, 23.0, owned[0].CurrentPeriod.UsedUSD)
		}
	}
	var completed int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM payment_orders WHERE status=$1`, OrderStatusCompleted).Scan(&completed))
	require.Equal(t, 7, completed, "group settlement does not change payment states")
}
