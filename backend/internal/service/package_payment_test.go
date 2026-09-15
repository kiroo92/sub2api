//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestPackageRefundDispatchRejectsBeforeProviderOrBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	u, err := client.User.Create().SetEmail("package-refund@example.com").SetPasswordHash("hash").Save(ctx)
	require.NoError(t, err)
	o, err := client.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName("package-user").SetAmount(385).SetPayAmount(385).SetFeeRate(0).SetRechargeCode("PACKAGE-REFUND").SetOutTradeNo("sub2_package_refund").SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("package-trade").SetOrderType(OrderTypePackage).SetStatus(OrderStatusCompleted).SetExpiresAt(time.Now()).SetPaidAt(time.Now()).SetClientIP("127.0.0.1").SetSrcHost("example.com").Save(ctx)
	require.NoError(t, err)
	s := &PaymentService{entClient: client}
	_, _, err = s.PrepareRefund(ctx, o.ID, 385, "test", true, true)
	require.Equal(t, "PACKAGE_REFUND_DISABLED", infraerrors.Reason(err))
	_, err = s.ExecuteRefund(ctx, &RefundPlan{OrderID: o.ID, Order: o})
	require.Equal(t, "PACKAGE_REFUND_DISABLED", infraerrors.Reason(err))
	_, err = s.QueryAndFinalizeRefund(ctx, o.ID)
	require.Equal(t, "PACKAGE_REFUND_DISABLED", infraerrors.Reason(err))
	err = s.RequestRefund(ctx, o.ID, u.ID, "test")
	require.Error(t, err)
	current, err := client.PaymentOrder.Get(ctx, o.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, current.Status)
}

func TestPackageWechatResumeCarriesDedicatedIDs(t *testing.T) {
	u, err := buildWeChatPaymentOAuthStartURL(CreateOrderRequest{OrderType: OrderTypePackage, PackagePlanID: 17, GroupBuyID: 23, PaymentType: payment.TypeWxpay}, "snsapi_base")
	require.NoError(t, err)
	require.Contains(t, u, "package_plan_id=17")
	require.Contains(t, u, "group_buy_id=23")
	s := NewPaymentResumeService([]byte("package-resume-signing-key-long-enough"))
	token, err := s.CreateWeChatPaymentResumeToken(WeChatPaymentResumeClaims{OpenID: "opaque-openid", OrderType: OrderTypePackage, PackagePlanID: 17, GroupBuyID: 23})
	require.NoError(t, err)
	claims, err := s.ParseWeChatPaymentResumeToken(token)
	require.NoError(t, err)
	require.Equal(t, int64(17), claims.PackagePlanID)
	require.Equal(t, int64(23), claims.GroupBuyID)
	require.Zero(t, claims.PlanID)
}
