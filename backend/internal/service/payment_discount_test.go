//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionDiscountPrice(t *testing.T) {
	for _, tc := range []struct {
		kind               string
		price, value, want float64
	}{
		{"percentage", 100, 80, 80}, {"fixed_amount", 100, 15, 85}, {"percentage", 9.99, 85, 8.49},
		{"fixed_amount", 100, 100, 0}, {"fixed_amount", 100, 101, 0}, {"percentage", 0.01, 1, 0},
		{"percentage", 100, 100, 0}, {"percentage", 100, math.NaN(), 0}, {"fixed_amount", 100, -1, 0},
	} {
		amount, err := discountPrice(tc.price, tc.kind, tc.value)
		if tc.want == 0 {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.want, amount)
		}
	}
	_, payable, err := calculateCreateOrderPayAmountForOrderType(80, 2, "CNY", payment.OrderTypeSubscription, 7)
	require.NoError(t, err)
	require.Equal(t, 571.2, payable)
	require.Equal(t, 571.2, calculateGatewayRefundAmount(80, payable, 80, "CNY"))
	require.Equal(t, 285.6, calculateGatewayRefundAmount(80, payable, 40, "CNY"))
	require.Error(t, discountExpectedAmount(nil, 80))
	accepted := 80.0
	require.NoError(t, discountExpectedAmount(&accepted, 80))
	require.Error(t, discountExpectedAmount(&accepted, 81))
}

func discountFixture(t *testing.T, client *dbent.Client) (*PaymentService, *dbent.SubscriptionPlan, *User, *dbent.SubscriptionDiscountCode) {
	t.Helper()
	ctx := context.Background()
	u, err := client.User.Create().SetEmail("discount@example.com").SetPasswordHash("test").SetUsername("discount").Save(ctx)
	require.NoError(t, err)
	g, err := client.Group.Create().SetName("discount-test").SetSubscriptionType(SubscriptionTypeSubscription).Save(ctx)
	require.NoError(t, err)
	plan, err := client.SubscriptionPlan.Create().SetName("monthly").SetGroupID(g.ID).SetPrice(100).SetValidityDays(30).Save(ctx)
	require.NoError(t, err)
	plan, err = client.SubscriptionPlan.Get(ctx, plan.ID) // Use DB timestamp precision, like normal checkout.
	require.NoError(t, err)
	config := NewPaymentConfigService(client, nil, nil)
	code, err := config.SaveDiscountCode(ctx, 0, SaveDiscountCodeRequest{Code: "vip80", DiscountType: "percentage", DiscountValue: 80, PlanIDs: []int64{plan.ID}, Enabled: true, MaxUses: 1, PerUserLimit: 1})
	require.NoError(t, err)
	return &PaymentService{entClient: client, configService: config}, plan, &User{ID: u.ID, Email: u.Email, Username: u.Username}, code
}

func discountedOrder(t *testing.T, svc *PaymentService, plan *dbent.SubscriptionPlan, user *User) *dbent.PaymentOrder {
	t.Helper()
	d, err := subscriptionDiscount(context.Background(), svc.entClient, user.ID, plan, " vip80 ", false)
	require.NoError(t, err)
	o, err := svc.createOrderInTx(context.Background(), CreateOrderRequest{UserID: user.ID, PlanID: plan.ID, OrderType: payment.OrderTypeSubscription, PaymentType: payment.TypeAlipay, CouponCode: "VIP80", discount: d}, user, plan, &PaymentConfig{MaxPendingOrders: 100}, d.Amount, d.Amount, 0, d.Amount, nil)
	require.NoError(t, err)
	return o
}

type discountUnavailableGroup struct{ GroupRepository }

func TestSubscriptionDiscountMultiplePlansAndPurchases(t *testing.T) {
	ctx := context.Background()
	svc, first, user, code := discountFixture(t, newPaymentOrderLifecycleTestClient(t))
	second, err := svc.entClient.SubscriptionPlan.Create().SetName("quarterly").SetGroupID(first.GroupID).SetPrice(200).SetValidityDays(90).Save(ctx)
	require.NoError(t, err)
	second, err = svc.entClient.SubscriptionPlan.Get(ctx, second.ID)
	require.NoError(t, err)
	settings := SaveDiscountCodeRequest{Code: code.Code, DiscountType: "percentage", DiscountValue: 80, PlanIDs: []int64{first.ID, second.ID}, Enabled: true, MaxUses: 0, PerUserLimit: 2}
	_, err = svc.configService.SaveDiscountCode(ctx, code.ID, settings)
	require.NoError(t, err)
	a := discountedOrder(t, svc, first, user)
	b := discountedOrder(t, svc, second, user)
	require.NotEqual(t, a.ID, b.ID)
	require.Equal(t, a.DiscountCodeID, b.DiscountCodeID)
	require.Equal(t, 80.0, a.Amount)
	require.Equal(t, 160.0, b.Amount)
	_, err = subscriptionDiscount(ctx, svc.entClient, user.ID, first, code.Code, false)
	require.Error(t, err, "per-user uses are shared across all plans for this code")
	settings.PerUserLimit = 0
	_, err = svc.configService.SaveDiscountCode(ctx, code.ID, settings)
	require.NoError(t, err)
	c := discountedOrder(t, svc, first, user)
	require.NotEqual(t, a.ID, c.ID, "the same subscription plan can be bought with the code again")
	_, err = subscriptionDiscount(ctx, svc.entClient, user.ID, &dbent.SubscriptionPlan{ID: 999, Price: 50}, code.Code, false)
	require.Error(t, err, "unselected plans are excluded")
	settings.PlanIDs = nil
	_, err = svc.configService.SaveDiscountCode(ctx, code.ID, settings)
	require.NoError(t, err)
	d, err := subscriptionDiscount(ctx, svc.entClient, user.ID, &dbent.SubscriptionPlan{ID: 999, Price: 50}, code.Code, false)
	require.NoError(t, err, "an empty plan selection allows all plans")
	require.Equal(t, 40.0, d.Amount)
}

func (discountUnavailableGroup) GetByID(context.Context, int64) (*Group, error) {
	return nil, errors.New("test: delivery unavailable")
}

func TestSubscriptionDiscountOrderLifecycle(t *testing.T) {
	ctx := context.Background()
	svc, plan, user, code := discountFixture(t, newPaymentOrderLifecycleTestClient(t))
	o := discountedOrder(t, svc, plan, user)
	require.Equal(t, 80.0, o.Amount)
	require.Equal(t, 20.0, PaymentOrderDiscount(o).DiscountAmount)
	_, err := subscriptionDiscount(ctx, svc.entClient, user.ID, plan, "VIP80", false)
	require.Error(t, err, "pending orders reserve the scarce use")
	_, err = svc.acquirePaymentFulfillmentLease(ctx, o)
	require.Error(t, err, "an unpaid failed discounted order cannot be delivered by an admin retry")
	// Missing closure evidence must preserve capacity even when cancellation returns nil.
	provider := &paymentOrderLifecycleQueryProvider{resp: &payment.QueryOrderResponse{Status: payment.ProviderStatusPending, TradeNo: o.OutTradeNo}}
	svc.registry = payment.NewRegistry()
	svc.registry.Register(provider)
	svc.providersLoaded = true
	o, err = svc.entClient.PaymentOrder.UpdateOneID(o.ID).SetDiscountState(discountReserved).SetStatus(OrderStatusCancelled).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.reconcileDiscountOrder(ctx, o))
	current, err := svc.entClient.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, discountReserved, current.DiscountState)
	provider.resp.Closed = true
	require.NoError(t, svc.reconcileDiscountOrder(ctx, o))
	require.NoError(t, svc.reconcileDiscountOrder(ctx, o))
	current, err = svc.entClient.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, discountReleased, current.DiscountState)
	paid := discountedOrder(t, svc, plan, user)
	svc.groupRepo = discountUnavailableGroup{}
	// Successful payment is counted once even if entitlement delivery fails and retries.
	require.Error(t, svc.toPaid(ctx, paid, "paid-discount", 80, payment.TypeAlipay))
	require.Error(t, svc.toPaid(ctx, paid, "paid-discount", 80, payment.TypeAlipay))
	paid, err = svc.entClient.PaymentOrder.Get(ctx, paid.ID)
	require.NoError(t, err)
	require.Equal(t, discountConsumed, paid.DiscountState)
	require.NotNil(t, paid.PaidAt)
	_, err = svc.entClient.SubscriptionDiscountCode.UpdateOneID(code.ID).SetDiscountValue(50).SetEnabled(false).Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 80.0, PaymentOrderDiscount(paid).Amount, "history must not reprice when the code changes")
	_, err = svc.entClient.PaymentOrder.UpdateOneID(paid.ID).SetStatus(OrderStatusRefunded).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, svc.releasePaymentDiscount(ctx, paid))
	paid, err = svc.entClient.PaymentOrder.Get(ctx, paid.ID)
	require.NoError(t, err)
	require.Equal(t, discountConsumed, paid.DiscountState, "refunds never restore a consumed use")
}

func TestSubscriptionDiscountValidation(t *testing.T) {
	ctx := context.Background()
	svc, plan, user, code := discountFixture(t, newPaymentOrderLifecycleTestClient(t))
	_, err := svc.configService.SaveDiscountCode(ctx, 0, SaveDiscountCodeRequest{Code: "VIP80", DiscountType: "percentage", DiscountValue: 50, Enabled: true})
	require.Error(t, err, "normalized code names must be unique")
	_, err = subscriptionDiscount(ctx, svc.entClient, user.ID, &dbent.SubscriptionPlan{ID: 999, Price: 100}, code.Code, false)
	require.Error(t, err)
	_, err = svc.entClient.SubscriptionDiscountCode.UpdateOneID(code.ID).SetExpiresAt(time.Now().Add(-time.Minute)).Save(ctx)
	require.NoError(t, err)
	_, err = subscriptionDiscount(ctx, svc.entClient, user.ID, plan, code.Code, false)
	require.Error(t, err)
	_, err = svc.entClient.SubscriptionDiscountCode.UpdateOneID(code.ID).ClearExpiresAt().SetEnabled(false).Save(ctx)
	require.NoError(t, err)
	_, err = subscriptionDiscount(ctx, svc.entClient, user.ID, plan, code.Code, false)
	require.Error(t, err)
	_, err = svc.entClient.SubscriptionDiscountCode.UpdateOneID(code.ID).SetEnabled(true).SetMaxUses(0).Save(ctx)
	require.NoError(t, err)
	discountedOrder(t, svc, plan, user)
	_, err = subscriptionDiscount(ctx, svc.entClient, user.ID, plan, code.Code, false)
	require.Error(t, err, "per-user limit applies even without a global cap")
	_, err = subscriptionDiscount(ctx, svc.entClient, user.ID+1, plan, code.Code, false)
	require.NoError(t, err, "another user has their own allowance")
	require.NoError(t, svc.entClient.SubscriptionPlan.DeleteOneID(plan.ID).Exec(ctx))
	_, err = svc.configService.SaveDiscountCode(ctx, code.ID, SaveDiscountCodeRequest{Code: code.Code, DiscountType: "percentage", DiscountValue: 80, PlanIDs: []int64{plan.ID}, Enabled: false, PerUserLimit: 1})
	require.NoError(t, err, "removing a plan must not prevent disabling its old code")
}
