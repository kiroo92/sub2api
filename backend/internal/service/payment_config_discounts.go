package service

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/subscriptiondiscountcode"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var subscriptionDiscountCodePattern = regexp.MustCompile(`^[A-Z0-9_-]{1,64}$`)

type SaveDiscountCodeRequest struct {
	Code          string     `json:"code"`
	DiscountType  string     `json:"discount_type"`
	DiscountValue float64    `json:"discount_value"`
	PlanIDs       []int64    `json:"plan_ids"`
	Enabled       bool       `json:"enabled"`
	ExpiresAt     *time.Time `json:"expires_at"`
	MaxUses       int        `json:"max_uses"`
	PerUserLimit  int        `json:"per_user_limit"`
}

type DiscountCodeResult struct {
	*dbent.SubscriptionDiscountCode
	ReservedCount int `json:"reserved_count"`
	UsedCount     int `json:"used_count"`
}

func (s *PaymentConfigService) SaveDiscountCode(ctx context.Context, id int64, req SaveDiscountCodeRequest) (*dbent.SubscriptionDiscountCode, error) {
	req.Code = strings.ToUpper(strings.TrimSpace(req.Code))
	var current *dbent.SubscriptionDiscountCode
	if id != 0 {
		var err error
		current, err = s.entClient.SubscriptionDiscountCode.Get(ctx, id)
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("COUPON_NOT_FOUND", "discount code does not exist")
		}
		if err != nil {
			return nil, err
		}
		if current.Code != req.Code {
			return nil, infraerrors.BadRequest("COUPON_CODE_IMMUTABLE", "create a new code instead of renaming an existing code")
		}
	}
	if !subscriptionDiscountCodePattern.MatchString(req.Code) || !isValidProviderAmount(req.DiscountValue) || req.DiscountValue >= 1e15 || req.MaxUses < 0 || req.PerUserLimit < 0 {
		return nil, infraerrors.BadRequest("COUPON_INVALID", "invalid code, discount value or usage limit")
	}
	value := decimal.NewFromFloat(req.DiscountValue)
	if !value.Equal(value.Round(2)) || req.DiscountType != "percentage" && req.DiscountType != "fixed_amount" || req.DiscountType == "percentage" && req.DiscountValue >= 100 {
		return nil, infraerrors.BadRequest("COUPON_INVALID", "use a positive amount or a payable percentage below 100, with at most two decimal places")
	}
	slices.Sort(req.PlanIDs)
	req.PlanIDs = slices.Compact(req.PlanIDs)
	if req.PlanIDs == nil {
		req.PlanIDs = []int64{}
	}
	if len(req.PlanIDs) > 0 {
		newIDs := make([]int64, 0, len(req.PlanIDs))
		for _, planID := range req.PlanIDs {
			if current == nil || !slices.Contains(current.PlanIds, planID) {
				newIDs = append(newIDs, planID)
			}
		}
		n, err := s.entClient.SubscriptionPlan.Query().Where(subscriptionplan.IDIn(newIDs...)).Count(ctx)
		if err != nil {
			return nil, err
		}
		if n != len(newIDs) {
			return nil, infraerrors.BadRequest("COUPON_PLAN_INVALID", "one or more subscription plans do not exist")
		}
	}
	var result *dbent.SubscriptionDiscountCode
	var err error
	if id == 0 {
		result, err = s.entClient.SubscriptionDiscountCode.Create().SetCode(req.Code).SetDiscountType(req.DiscountType).SetDiscountValue(req.DiscountValue).
			SetPlanIds(req.PlanIDs).SetEnabled(req.Enabled).SetNillableExpiresAt(req.ExpiresAt).SetMaxUses(req.MaxUses).SetPerUserLimit(req.PerUserLimit).Save(ctx)
	} else {
		update := s.entClient.SubscriptionDiscountCode.UpdateOneID(id).SetDiscountType(req.DiscountType).SetDiscountValue(req.DiscountValue).
			SetPlanIds(req.PlanIDs).SetEnabled(req.Enabled).SetMaxUses(req.MaxUses).SetPerUserLimit(req.PerUserLimit)
		if req.ExpiresAt == nil {
			update.ClearExpiresAt()
		} else {
			update.SetExpiresAt(*req.ExpiresAt)
		}
		result, err = update.Save(ctx)
	}
	if dbent.IsConstraintError(err) {
		return nil, infraerrors.Conflict("COUPON_DUPLICATE", "discount code already exists")
	}
	return result, err
}

func (s *PaymentConfigService) ListDiscountCodes(ctx context.Context, page, size int, search string) ([]DiscountCodeResult, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	q := s.entClient.SubscriptionDiscountCode.Query()
	if search = strings.TrimSpace(search); search != "" {
		q.Where(subscriptiondiscountcode.CodeContainsFold(search))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	codes, err := q.Order(dbent.Desc(subscriptiondiscountcode.FieldID)).Limit(size).Offset((page - 1) * size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	results := make([]DiscountCodeResult, 0, len(codes))
	for _, code := range codes {
		q := s.entClient.PaymentOrder.Query().Where(paymentorder.DiscountCodeIDEQ(code.ID))
		used, err := q.Clone().Where(paymentorder.DiscountStateEQ(discountConsumed)).Count(ctx)
		if err != nil {
			return nil, 0, err
		}
		reserved, err := q.Where(paymentorder.DiscountStateIn(discountCreating, discountReserved)).Count(ctx)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, DiscountCodeResult{SubscriptionDiscountCode: code, UsedCount: used, ReservedCount: reserved})
	}
	return results, total, nil
}
