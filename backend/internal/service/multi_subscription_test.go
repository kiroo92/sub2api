//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestAllSubscriptionsModeSwitchClearsFixedGroupAtomically(t *testing.T) {
	groupID := int64(12)
	mode := APIKeyRoutingAllSubscriptions
	svc, repo := newUpdateFieldsAPIKeyService(&APIKey{ID: 1, UserID: 7, GroupID: &groupID, RoutingMode: APIKeyRoutingFixedGroup, Group: &Group{ID: groupID}})
	key, err := svc.Update(context.Background(), 1, 7, UpdateAPIKeyRequest{RoutingMode: &mode})
	require.NoError(t, err)
	require.Nil(t, key.GroupID)
	require.Nil(t, key.Group)
	require.True(t, key.UsesAllSubscriptions())
	require.Equal(t, []APIKeyUpdateFields{{RoutingMode: true, GroupID: true}}, repo.updateFields)
	_, err = svc.Update(context.Background(), 1, 7, UpdateAPIKeyRequest{RoutingMode: &mode, GroupID: &groupID})
	require.ErrorIs(t, err, ErrAPIKeyGroupConflict)
	require.Len(t, repo.updateFields, 1)
	require.ErrorIs(t, validateAPIKeyRoutingMode("all_packages", nil), ErrAPIKeyRoutingMode)
	key.User = &User{ID: 7}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	reloaded := svc.snapshotToAPIKey("test", snapshot)
	require.True(t, reloaded.UsesAllSubscriptions())
	require.Nil(t, reloaded.GroupID)
	require.Nil(t, reloaded.SubscriptionRate, "request-local prices must never enter auth cache")
}

func TestSubscriptionRefundTargetsOnlyPurchasedRecord(t *testing.T) {
	groupID, days := int64(3), 30
	now := time.Now()
	repo := &independentSubscriptionsRepo{rows: []UserSubscription{
		{ID: 1, UserID: 7, GroupID: groupID, Status: SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour), Notes: "payment order 123"},
		{ID: 2, UserID: 7, GroupID: groupID, Status: SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour), Notes: "payment order 12"},
	}}
	svc := &PaymentService{subscriptionSvc: NewSubscriptionService(nil, repo, nil, nil, nil)}
	order := &dbent.PaymentOrder{ID: 12, UserID: 7, OrderType: payment.OrderTypeSubscription, SubscriptionGroupID: &groupID, SubscriptionDays: &days}
	plan := &RefundPlan{}
	require.Nil(t, svc.prepDeduct(context.Background(), order, plan, false))
	require.Equal(t, int64(2), plan.SubscriptionID)
	require.Equal(t, days, plan.SubDaysToDeduct)
	repo.rows[1].ExpiresAt = now.Add(-time.Hour)
	plan = &RefundPlan{}
	require.Nil(t, svc.prepDeduct(context.Background(), order, plan, false))
	require.Zero(t, plan.SubDaysToDeduct, "expired purchase must not shorten another subscription")
}

type refundSubscriptionRepo struct{ lockingRenewalRepo }

func (r *refundSubscriptionRepo) GetByIDIncludeDeleted(ctx context.Context, id int64) (*UserSubscription, error) {
	return r.GetByID(ctx, id)
}
func (r *refundSubscriptionRepo) Restore(ctx context.Context, id int64, status string) (*UserSubscription, error) {
	r.current.DeletedAt, r.current.Status = nil, status
	return r.GetByID(ctx, id)
}

func TestSubscriptionRefundRollbackPreservesConcurrentExtensionAndUsage(t *testing.T) {
	now := time.Now()
	before := UserSubscription{ID: 3, UserID: 7, GroupID: 8, Status: SubscriptionStatusActive, ExpiresAt: now.AddDate(0, 0, 60)}
	for _, revoked := range []bool{false, true} {
		repo := &refundSubscriptionRepo{lockingRenewalRepo: lockingRenewalRepo{current: before}}
		if revoked {
			repo.current.DeletedAt = &now
		} else {
			repo.current.ExpiresAt = before.ExpiresAt.AddDate(0, 0, -30)
		}
		repo.current.ExpiresAt = repo.current.ExpiresAt.AddDate(0, 0, 7)
		repo.current.DailyUsageUSD = 4
		svc := &PaymentService{subscriptionSvc: NewSubscriptionService(nil, repo, nil, nil, nil)}
		plan := &RefundPlan{SubscriptionID: 3, SubDaysToDeduct: 30, DeductionType: payment.DeductionTypeSubscription, SubscriptionBeforeRefund: &before, SubscriptionRevokedByRefund: revoked}
		require.True(t, svc.RollbackRefund(context.Background(), plan, nil))
		require.WithinDuration(t, before.ExpiresAt.AddDate(0, 0, 7), repo.current.ExpiresAt, time.Millisecond)
		require.Equal(t, 4.0, repo.current.DailyUsageUSD)
		require.Nil(t, repo.current.DeletedAt)
	}
}

type independentSubscriptionsRepo struct {
	UserSubscriptionRepository
	rows   []UserSubscription
	readID int64
}

func (r *independentSubscriptionsRepo) Create(_ context.Context, sub *UserSubscription) error {
	sub.ID = int64(len(r.rows) + 1)
	r.rows = append(r.rows, *sub)
	return nil
}

func (r *independentSubscriptionsRepo) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	r.readID = id
	for _, sub := range r.rows {
		if sub.ID == id {
			return &sub, nil
		}
	}
	return nil, ErrSubscriptionNotFound
}

func (r *independentSubscriptionsRepo) ListByUserID(_ context.Context, id int64) ([]UserSubscription, error) {
	return r.rows, nil
}

func (r *independentSubscriptionsRepo) ListActiveByUserID(_ context.Context, id int64) ([]UserSubscription, error) {
	return append([]UserSubscription(nil), r.rows...), nil
}

func TestAllSubscriptionsSelectsInOrderWithoutMutatingKeyOrFallback(t *testing.T) {
	now := time.Now()
	limit, fallback := 10.0, int64(99)
	group := &Group{ID: 3, Status: StatusActive, Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit, FallbackGroupID: &fallback}
	first := UserSubscription{ID: 1, UserID: 7, GroupID: 3, Group: group, Status: SubscriptionStatusActive, StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now, DailyUsageUSD: 10}
	second := first
	second.ID, second.DailyUsageUSD = 2, 0
	repo := &independentSubscriptionsRepo{rows: []UserSubscription{first, second}}
	svc := NewSubscriptionService(nil, repo, nil, nil, nil)
	t.Cleanup(svc.Stop)
	key := &APIKey{UserID: 7, User: &User{ID: 7, Balance: 1000}, RoutingMode: APIKeyRoutingAllSubscriptions}
	request := SubscriptionRequest{Path: "/v1/responses", Models: []string{"gpt-5"}}
	selected, sub, err := svc.SelectForRequest(context.Background(), key, request)
	require.NoError(t, err)
	require.Equal(t, int64(2), sub.ID)
	require.Nil(t, key.GroupID)
	require.Nil(t, selected.Group.FallbackGroupID)
	require.Equal(t, &fallback, group.FallbackGroupID)
	repo.rows[1].DailyUsageUSD = limit
	_, _, err = svc.SelectForRequest(context.Background(), key, request)
	require.ErrorIs(t, err, ErrNoEligibleSubscription)
	discovery, sub, err := svc.SelectForRequest(context.Background(), key, SubscriptionRequest{Discovery: true})
	require.NoError(t, err)
	require.Nil(t, sub)
	require.Len(t, discovery.SubscriptionGroups, 2, "temporarily exhausted subscriptions remain discoverable")
	_, _, err = svc.SelectForRequest(context.Background(), key, SubscriptionRequest{Path: "/v1beta/models/gemini:generateContent", Models: []string{"gemini"}})
	require.ErrorIs(t, err, ErrNoEligibleSubscription)
}

func TestPurchasedSubscriptionsAreIndependentAndOrderNotesAreExact(t *testing.T) {
	repo := &independentSubscriptionsRepo{}
	groups := &subscriptionGroupRepoStub{group: &Group{ID: 3, SubscriptionType: SubscriptionTypeSubscription}}
	svc := NewSubscriptionService(groups, repo, nil, nil, nil)
	t.Cleanup(svc.Stop)
	first, err := svc.CreatePurchasedSubscription(context.Background(), &AssignSubscriptionInput{UserID: 7, GroupID: 3, ValidityDays: 1})
	require.NoError(t, err)
	second, err := svc.CreatePurchasedSubscription(context.Background(), &AssignSubscriptionInput{UserID: 7, GroupID: 3, ValidityDays: 30})
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	require.Equal(t, first.ExpiresAt, repo.rows[0].ExpiresAt)
	require.Greater(t, second.ExpiresAt.Unix(), first.ExpiresAt.Unix())
	require.False(t, hasPaymentSubscriptionOrderNote("payment order 123", paymentSubscriptionOrderNote(12)))
	require.True(t, hasPaymentSubscriptionOrderNote("admin note\r\npayment order 12\r\n", paymentSubscriptionOrderNote(12)))
}

func TestAllSubscriptionsBillingUsesSelectedIDAndNeverBalance(t *testing.T) {
	now := time.Now()
	limit := 10.0
	group := &Group{ID: 3, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit}
	first := UserSubscription{ID: 1, UserID: 7, GroupID: 3, Status: SubscriptionStatusActive, StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), DailyUsageUSD: 10}
	second := first
	second.ID, second.DailyUsageUSD = 2, 0
	repo := &independentSubscriptionsRepo{rows: []UserSubscription{first, second}}
	svc := &BillingCacheService{cfg: &config.Config{}, subRepo: repo}
	key := &APIKey{RoutingMode: APIKeyRoutingAllSubscriptions}
	user := &User{ID: 7, Balance: 1000}
	require.NoError(t, svc.CheckBillingEligibility(context.Background(), user, key, group, &second, ""))
	require.Equal(t, int64(2), repo.readID)
	require.ErrorIs(t, svc.CheckBillingEligibility(context.Background(), user, key, group, &first, ""), ErrDailyLimitExceeded)
	require.ErrorIs(t, svc.CheckBillingEligibility(context.Background(), user, key, group, nil, ""), ErrSubscriptionInvalid)
	second.UserID = 8
	require.ErrorIs(t, svc.CheckBillingEligibility(context.Background(), user, key, group, &second, ""), ErrSubscriptionInvalid)
}
