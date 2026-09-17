package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type teamRoutingRepo struct {
	TeamRepository
	attribution *TeamAttribution
	err         error
	admitted    int
}

func (r *teamRoutingRepo) KeyAttribution(context.Context, int64) (*TeamAttribution, error) {
	if r.attribution == nil {
		return nil, r.err
	}
	a := *r.attribution
	return &a, r.err
}
func (r *teamRoutingRepo) Admit(context.Context, *TeamAttribution, int64) error {
	r.admitted++
	return r.err
}

type teamRoutingUsers struct {
	UserRepository
	users map[int64]*User
}

func (r *teamRoutingUsers) GetByID(_ context.Context, id int64) (*User, error) {
	return r.users[id], nil
}

type teamRoutingGroups struct {
	GroupRepository
	groups map[int64]*Group
}

type teamRoutingSubscriptions struct {
	UserSubscriptionRepository
	rows []UserSubscription
}

func (r *teamRoutingSubscriptions) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return append([]UserSubscription(nil), r.rows...), nil
}

func (r *teamRoutingGroups) GetByID(_ context.Context, id int64) (*Group, error) {
	return r.groups[id], nil
}

type teamUsageWriter struct {
	UsageLogRepository
	synchronous, queued int
}

func (w *teamUsageWriter) Create(context.Context, *UsageLog) (bool, error) {
	w.synchronous++
	return true, nil
}
func (w *teamUsageWriter) CreateBestEffort(context.Context, *UsageLog) error { w.queued++; return nil }
func TestTeamUsageCompletesBeforeCleanup(t *testing.T) {
	writer := &teamUsageWriter{}
	writeUsageLogBestEffort(context.WithValue(context.Background(), ctxkey.TeamBilling, true), writer, &UsageLog{}, "test")
	require.Equal(t, 1, writer.synchronous)
	require.Zero(t, writer.queued)
}

func TestTeamRoutingUsesOwnerSourcesAndKeepsPersonalKeysUnchanged(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	limit := 10.0
	subGroup := &Group{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &limit}
	balanceGroup := &Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 2}
	sub := UserSubscription{ID: 11, UserID: 7, GroupID: 1, Group: subGroup, Status: SubscriptionStatusActive, StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now, DailyUsageUSD: 10}
	subs := &teamRoutingSubscriptions{rows: []UserSubscription{sub}}
	teams := &teamRoutingRepo{attribution: &TeamAttribution{TeamID: 1, MemberID: 3, MemberUserID: 8, OwnerID: 7, GroupIDs: []int64{1, 2}}}
	owner := &User{ID: 7, Status: StatusActive, Balance: 100}
	member := &User{ID: 8, Status: StatusActive, Balance: 500}
	svc := &SubscriptionService{now: time.Now, teamRepo: teams, teamUsers: &teamRoutingUsers{users: map[int64]*User{7: owner, 8: member}}, groupRepo: &teamRoutingGroups{groups: map[int64]*Group{1: subGroup, 2: balanceGroup}}, userSubRepo: subs}
	key := &APIKey{ID: 9, UserID: 7, User: owner, RoutingMode: "team"}
	req := SubscriptionRequest{Path: "/v1/responses", Models: []string{"gpt-5"}}
	selected, chosen, err := svc.SelectForRequest(ctx, key, req)
	require.NoError(t, err)
	require.Nil(t, chosen)
	require.Equal(t, int64(2), *selected.GroupID)
	require.Equal(t, int64(7), selected.UserID)
	require.Equal(t, int64(8), selected.Team.MemberUserID)
	require.Nil(t, key.Team)
	require.Nil(t, key.GroupID)
	require.Equal(t, 2.0, *selected.SubscriptionRate)
	require.Equal(t, 1, teams.admitted)
	subs.rows[0].DailyUsageUSD = 0
	owner.Balance = 0
	selected, chosen, err = svc.SelectForRequest(ctx, key, req)
	require.NoError(t, err)
	require.Equal(t, int64(11), chosen.ID)
	require.Equal(t, int64(1), *selected.GroupID)
	teams.err = ErrTeamLimit
	_, _, err = svc.SelectForRequest(ctx, key, req)
	require.ErrorIs(t, err, ErrTeamLimit)
	teams.err = nil
	teams.attribution = nil
	_, _, err = svc.SelectForRequest(ctx, key, req)
	require.ErrorIs(t, err, ErrTeamUnavailable, "deleted team cannot turn cached team key into personal key")
	personal := &APIKey{RoutingMode: APIKeyRoutingFixedGroup, User: member, UserID: 8}
	unchanged, err := svc.PrepareTeamKey(ctx, personal)
	require.NoError(t, err)
	require.Same(t, personal, unchanged)
}
