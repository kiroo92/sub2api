package service

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/subscriptiondiscountcode"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	discountCreating = "creating"
	discountReserved = "reserved"
	discountConsumed = "consumed"
	discountReleased = "released"
)

// PaymentDiscount is frozen on the order, in the plan's price units before FX and fees.
type PaymentDiscount struct {
	Version        int     `json:"version"`
	CodeID         int64   `json:"code_id"`
	Code           string  `json:"code"`
	Type           string  `json:"type"`
	Value          float64 `json:"value"`
	OriginalAmount float64 `json:"original_amount"`
	DiscountAmount float64 `json:"discount_amount"`
	Amount         float64 `json:"amount"`
	USDToCNYRate   float64 `json:"usd_to_cny_rate"`
}

type SubscriptionQuote struct {
	OriginalAmount float64          `json:"original_amount"`
	Amount         float64          `json:"amount"`
	PayAmount      float64          `json:"pay_amount"`
	FeeRate        float64          `json:"fee_rate"`
	Currency       string           `json:"currency"`
	Discount       *PaymentDiscount `json:"discount,omitempty"`
}

func discountPrice(price float64, kind string, value float64) (float64, error) {
	if !isValidProviderAmount(price) || !isValidProviderAmount(value) {
		return 0, infraerrors.BadRequest("COUPON_INVALID", "invalid discount amount")
	}
	base, v := decimal.NewFromFloat(price), decimal.NewFromFloat(value)
	var amount decimal.Decimal
	switch kind {
	case "percentage": // 80 means pay 80%, i.e. 八折.
		if value >= 100 {
			return 0, infraerrors.BadRequest("COUPON_INVALID", "percentage must be between 0 and 100")
		}
		amount = base.Mul(v).Div(decimal.NewFromInt(100)).Round(2)
	case "fixed_amount":
		amount = base.Sub(v).Round(2)
	default:
		return 0, infraerrors.BadRequest("COUPON_INVALID", "unknown discount type")
	}
	if !amount.IsPositive() || !amount.LessThan(base) {
		return 0, infraerrors.BadRequest("COUPON_AMOUNT_INVALID", "discount must reduce the price and leave a positive amount")
	}
	return amount.InexactFloat64(), nil
}

func loadDiscount(ctx context.Context, client *dbent.Client, code string, lock bool) (*dbent.SubscriptionDiscountCode, error) {
	q := client.SubscriptionDiscountCode.Query().Where(subscriptiondiscountcode.CodeEQ(strings.ToUpper(strings.TrimSpace(code))))
	if lock && paymentAuditDialect(client) == dialect.Postgres {
		q.ForUpdate()
	}
	c, err := q.Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, infraerrors.BadRequest("COUPON_NOT_FOUND", "discount code does not exist")
	}
	return c, err
}

func subscriptionDiscount(ctx context.Context, client *dbent.Client, uid int64, plan *dbent.SubscriptionPlan, code string, lock bool) (*PaymentDiscount, error) {
	if !subscriptionDiscountCodePattern.MatchString(strings.ToUpper(strings.TrimSpace(code))) {
		return nil, infraerrors.BadRequest("COUPON_INVALID", "invalid discount code")
	}
	c, err := loadDiscount(ctx, client, code, lock)
	if err != nil {
		return nil, err
	}
	if !c.Enabled {
		return nil, infraerrors.BadRequest("COUPON_DISABLED", "discount code is disabled")
	}
	if c.ExpiresAt != nil && !time.Now().Before(*c.ExpiresAt) {
		return nil, infraerrors.BadRequest("COUPON_EXPIRED", "discount code has expired")
	}
	if len(c.PlanIds) > 0 && !slices.Contains(c.PlanIds, plan.ID) {
		return nil, infraerrors.BadRequest("COUPON_PLAN_MISMATCH", "discount code does not apply to this plan")
	}
	q := client.PaymentOrder.Query().Where(paymentorder.DiscountCodeIDEQ(c.ID), paymentorder.DiscountStateIn(discountCreating, discountReserved, discountConsumed))
	if c.MaxUses > 0 {
		n, err := q.Clone().Count(ctx)
		if err != nil {
			return nil, err
		}
		if n >= c.MaxUses {
			return nil, infraerrors.Conflict("COUPON_LIMIT_REACHED", "discount code has no remaining uses (including pending orders)")
		}
	}
	if c.PerUserLimit > 0 {
		n, err := q.Where(paymentorder.UserIDEQ(uid)).Count(ctx)
		if err != nil {
			return nil, err
		}
		if n >= c.PerUserLimit {
			return nil, infraerrors.Conflict("COUPON_USER_LIMIT_REACHED", "your discount uses are exhausted (including pending orders)")
		}
	}
	amount, err := discountPrice(plan.Price, c.DiscountType, c.DiscountValue)
	if err != nil {
		return nil, err
	}
	return &PaymentDiscount{Version: 1, CodeID: c.ID, Code: c.Code, Type: c.DiscountType, Value: c.DiscountValue, OriginalAmount: plan.Price, Amount: amount,
		DiscountAmount: decimal.NewFromFloat(plan.Price).Sub(decimal.NewFromFloat(amount)).InexactFloat64()}, nil
}

func (d *PaymentDiscount) snapshot() map[string]any {
	// All fields are validated finite scalars; the same typed contract owns reading and writing.
	data, _ := json.Marshal(d)
	var snapshot map[string]any
	_ = json.Unmarshal(data, &snapshot)
	return snapshot
}

func PaymentOrderDiscount(o *dbent.PaymentOrder) *PaymentDiscount {
	if o == nil || o.DiscountCodeID == nil || len(o.DiscountSnapshot) == 0 {
		return nil
	}
	data, err := json.Marshal(o.DiscountSnapshot)
	if err != nil {
		return nil
	}
	var d PaymentDiscount
	if json.Unmarshal(data, &d) != nil || d.Version != 1 || d.CodeID != *o.DiscountCodeID {
		return nil
	}
	return &d
}

func (s *PaymentService) QuoteSubscription(ctx context.Context, req CreateOrderRequest) (*SubscriptionQuote, error) {
	req.OrderType = payment.OrderTypeSubscription
	if normalized := NormalizeVisibleMethod(req.PaymentType); normalized != "" {
		req.PaymentType = normalized
	} else {
		req.PaymentType = strings.TrimSpace(req.PaymentType)
	}
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, infraerrors.Forbidden("PAYMENT_DISABLED", "payment system is disabled")
	}
	plan, err := s.validateOrderInput(ctx, req, cfg)
	if err != nil {
		return nil, err
	}
	quote := &SubscriptionQuote{OriginalAmount: plan.Price, Amount: plan.Price, FeeRate: cfg.RechargeFeeRate}
	if strings.TrimSpace(req.CouponCode) != "" {
		quote.Discount, err = subscriptionDiscount(ctx, s.entClient, req.UserID, plan, req.CouponCode, false)
		if err != nil {
			return nil, err
		}
		quote.Discount.USDToCNYRate = cfg.SubscriptionUSDToCNYRate
		quote.Amount = quote.Discount.Amount
	}
	quote.Currency, err = s.configService.ValidateMethodCurrencyConsistency(ctx, req.PaymentType)
	if err != nil {
		return nil, err
	}
	_, quote.PayAmount, err = calculateCreateOrderPayAmountForOrderType(quote.Amount, cfg.RechargeFeeRate, quote.Currency, req.OrderType, cfg.SubscriptionUSDToCNYRate)
	if err != nil {
		return nil, err
	}
	// Reuse the existing method limits without selecting/advancing a payment provider.
	limits, err := s.configService.GetAvailableMethodLimits(ctx)
	if err != nil {
		return nil, err
	}
	limit, available := limits.Methods[req.PaymentType]
	if !available {
		return nil, infraerrors.BadRequest("PAYMENT_METHOD_UNAVAILABLE", "payment method is unavailable")
	}
	if limit.SingleMin > 0 && quote.PayAmount < limit.SingleMin || limit.SingleMax > 0 && quote.PayAmount > limit.SingleMax {
		return nil, infraerrors.BadRequest("COUPON_PAYMENT_LIMIT", "discounted amount is outside this payment method's limits")
	}
	return quote, nil
}

func discountExpectedAmount(expected *float64, actual float64) error {
	if expected == nil || !isValidProviderAmount(*expected) || math.Abs(*expected-actual) > 0.0000001 {
		return infraerrors.Conflict("CHECKOUT_PRICE_CHANGED", "checkout price changed; apply the discount again before paying")
	}
	return nil
}
