//go:build unit

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
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestPaymentDiscountPostgres(t *testing.T) {
	dsn := os.Getenv("PAYMENT_DISCOUNT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PAYMENT_DISCOUNT_TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	ctx := context.Background()
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("discount_test_%d", time.Now().UnixNano())
	_, err = base.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, e := base.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, e)
		require.NoError(t, base.Close())
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, client.Schema.Create(ctx))
	_, err = db.ExecContext(ctx, `DROP TABLE subscription_discount_codes; ALTER TABLE payment_orders DROP COLUMN discount_code_id, DROP COLUMN discount_state, DROP COLUMN discount_snapshot`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("243_subscription_discount_codes.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = db.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	svc, plan, user, code := discountFixture(t, client)
	d, err := subscriptionDiscount(ctx, client, user.ID, plan, code.Code, false)
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.createOrderInTx(ctx, CreateOrderRequest{UserID: user.ID, PlanID: plan.ID, OrderType: payment.OrderTypeSubscription, PaymentType: payment.TypeAlipay, CouponCode: code.Code, discount: d}, user, plan, &PaymentConfig{MaxPendingOrders: 100}, 80, 80, 0, 80, nil)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	require.Equal(t, 1, succeeded, "row locks must enforce the limit across concurrent transactions")
	order, err := client.PaymentOrder.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 80.0, order.Amount)
	require.Equal(t, 20.0, PaymentOrderDiscount(order).DiscountAmount)
	_, err = db.ExecContext(ctx, `UPDATE payment_orders SET discount_code_id = NULL WHERE id = $1`, order.ID)
	require.Error(t, err, "database rejects a detached usage snapshot")
	require.NoError(t, svc.releasePaymentDiscount(ctx, order))
	require.NoError(t, svc.releasePaymentDiscount(ctx, order))
	discountedOrder(t, svc, plan, user)
	rows, total, err := svc.configService.ListDiscountCodes(ctx, 1, 20, "")
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, rows[0].ReservedCount)
	_, err = db.ExecContext(ctx, string(migration))
	require.NoError(t, err, "replaying migration must preserve existing discount uses")
	latest, err := client.PaymentOrder.Query().Order(dbent.Desc("id")).First(ctx)
	require.NoError(t, err)
	svc.groupRepo = discountUnavailableGroup{}
	start = make(chan struct{})
	results = make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); <-start; results <- svc.releasePaymentDiscount(ctx, latest) }()
	go func() {
		defer wg.Done()
		<-start
		results <- svc.toPaid(ctx, latest, "race-payment", 80, payment.TypeAlipay)
	}()
	close(start)
	wg.Wait()
	close(results)
	for range results {
	} // Delivery is deliberately unavailable; assert durable state below.
	latest, err = client.PaymentOrder.Get(ctx, latest.ID)
	require.NoError(t, err)
	if latest.DiscountState == discountReleased {
		require.Nil(t, latest.PaidAt, "a released use must not also have an accepted payment")
	} else {
		require.Equal(t, discountConsumed, latest.DiscountState)
		require.NotNil(t, latest.PaidAt)
	}
}
