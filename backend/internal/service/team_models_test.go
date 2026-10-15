package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTeamModelValidation(t *testing.T) {
	allowed := []string{"gpt-5", "claude-sonnet-4"}
	require.NoError(t, ValidateTeamModels(nil, []string{"any-model"}, false))
	require.NoError(t, ValidateTeamModels(allowed, nil, true))
	require.NoError(t, ValidateTeamModels(allowed, allowed, false))
	for _, requested := range [][]string{nil, {""}, {"GPT-5"}, {"gpt-5-extra"}, {"gpt-5", "denied"}} {
		require.ErrorIs(t, ValidateTeamModels(allowed, requested, false), ErrTeamModelForbidden)
	}
	models, err := normalizeTeamModels([]string{" gpt-5 ", "gpt-5", "claude-sonnet-4"})
	require.NoError(t, err)
	require.Equal(t, allowed, models)
	models, err = normalizeTeamModels(nil)
	require.NoError(t, err)
	require.NotNil(t, models)
	for _, invalid := range []string{"", " ", "gpt-*", "a,b", "a b", "a\x00b", strings.Repeat("a", 201)} {
		_, err = normalizeTeamModels([]string{invalid})
		require.Error(t, err)
	}
	_, err = normalizeTeamModels(make([]string, 201))
	require.Error(t, err)
}

func TestTeamModelRestrictionsReloadBeforeRoutingAndAdmission(t *testing.T) {
	owner := &User{ID: 7, Status: StatusActive, Balance: 100}
	member := &User{ID: 8, Status: StatusActive}
	group := &Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive}
	repo := &teamRoutingRepo{attribution: &TeamAttribution{TeamID: 1, MemberID: 3, MemberUserID: 8, OwnerID: 7, GroupIDs: []int64{2}, AllowedModels: []string{"gpt-5"}}}
	svc := &SubscriptionService{now: time.Now, teamRepo: repo, teamUsers: &teamRoutingUsers{users: map[int64]*User{7: owner, 8: member}}, groupRepo: &teamRoutingGroups{groups: map[int64]*Group{2: group}}}
	// Cached credentials carry an outdated allowlist; fresh database state wins.
	key := &APIKey{ID: 9, RoutingMode: "team", Team: &TeamAttribution{AllowedModels: []string{"denied"}}}
	for _, req := range []SubscriptionRequest{
		{Path: "/v1/responses", Models: []string{"denied"}},
		{Path: "/v1/messages", Models: []string{"gpt-5", "denied"}},
		{Path: "/v1/responses", WebSocket: true, Models: []string{"denied"}},
		{Path: "/v1/responses"},
	} {
		_, _, err := svc.SelectForRequest(context.Background(), key, req)
		require.ErrorIs(t, err, ErrTeamModelForbidden)
		require.Zero(t, repo.admitted)
	}
	selected, _, err := svc.SelectForRequest(context.Background(), key, SubscriptionRequest{Path: "/v1/responses", Models: []string{"gpt-5"}})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5"}, selected.Team.RequestModels)
	require.Equal(t, 1, repo.admitted)
	repo.attribution.AllowedModels = []string{"new-model"}
	_, _, err = svc.SelectForRequest(context.Background(), selected, SubscriptionRequest{Path: "/v1/responses", Models: []string{"gpt-5"}, WebSocket: true})
	require.ErrorIs(t, err, ErrTeamModelForbidden)
	require.Equal(t, 1, repo.admitted)
}

func TestTeamModelDiscoveryFiltersOnlyTeamKeys(t *testing.T) {
	models := []string{"gpt-5", "denied"}
	key := &APIKey{RoutingMode: "team", Team: &TeamAttribution{AllowedModels: []string{"gpt-5"}}}
	require.Equal(t, []string{"gpt-5"}, FilterTeamModels(key, models))
	key.RoutingMode = APIKeyRoutingFixedGroup
	key.Team = nil
	require.Equal(t, models, FilterTeamModels(key, models))
	key.RoutingMode = "team"
	key.Team = &TeamAttribution{}
	require.Equal(t, models, FilterTeamModels(key, models))
}

type teamModelCatalogRepo struct {
	TeamRepository
	snapshot *TeamSnapshot
}

func (r *teamModelCatalogRepo) Snapshot(context.Context, int64) (*TeamSnapshot, error) {
	return r.snapshot, nil
}

type teamModelCatalogGroups struct {
	GroupRepository
	groups []Group
}

func (r *teamModelCatalogGroups) ListActive(context.Context) ([]Group, error) {
	return r.groups, nil
}

func TestTeamModelCatalogUsesConfiguredOwnerGroupsAndRejectsMembers(t *testing.T) {
	repo := &teamModelCatalogRepo{snapshot: &TeamSnapshot{Role: "member", Team: &Team{GroupIDs: []int64{2, 1, 4}}}}
	svc := NewTeamService(repo, nil, nil, nil, nil)
	_, err := svc.ModelGroups(context.Background(), 8)
	require.ErrorIs(t, err, ErrTeamForbidden, "member must be rejected before accessing model sources")
	repo.snapshot.Role = "owner"
	svc.keys = &APIKeyService{
		userRepo: &teamRoutingUsers{users: map[int64]*User{7: {ID: 7, Status: StatusActive}}},
		groupRepo: &teamModelCatalogGroups{groups: []Group{
			{ID: 1, Status: StatusActive}, {ID: 2, Status: StatusActive},
			{ID: 3, Status: StatusActive}, {ID: 4, Status: StatusActive, IsExclusive: true},
		}},
		userSubRepo: &teamRoutingSubscriptions{},
	}
	groups, err := svc.ModelGroups(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Equal(t, int64(2), groups[0].ID)
	require.Equal(t, int64(1), groups[1].ID)
	require.NotSame(t, groups[0], groups[1])
	repo.snapshot.Team.GroupIDs = nil
	groups, err = svc.ModelGroups(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, groups, 3, "unconfigured teams preview owner-accessible groups without selecting routing sources")
	require.Empty(t, repo.snapshot.Team.GroupIDs)
	require.Equal(t, int64(3), groups[2].ID)
}

type teamDiscoveryGroups struct {
	teamRoutingGroups
}

func (r *teamDiscoveryGroups) ListActive(context.Context) ([]Group, error) {
	out := make([]Group, 0, len(r.groups))
	for _, group := range r.groups {
		out = append(out, *group)
	}
	return out, nil
}

func TestTeamKeyDiscoveryBeforePoolSelectionPreservesMemberRestrictions(t *testing.T) {
	owner := &User{ID: 7, Status: StatusActive}
	member := &User{ID: 8, Status: StatusActive}
	group := &Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive}
	repo := &teamRoutingRepo{attribution: &TeamAttribution{TeamID: 1, MemberID: 3, MemberUserID: 8, OwnerID: 7, AllowedModels: []string{"gpt-5"}}}
	svc := &SubscriptionService{
		now: time.Now, teamRepo: repo,
		teamUsers:   &teamRoutingUsers{users: map[int64]*User{7: owner, 8: member}},
		groupRepo:   &teamDiscoveryGroups{teamRoutingGroups{groups: map[int64]*Group{2: group}}},
		userSubRepo: &teamRoutingSubscriptions{},
	}
	key := &APIKey{ID: 9, RoutingMode: "team"}
	selected, _, err := svc.SelectForRequest(context.Background(), key, SubscriptionRequest{Path: "/v1/models", Discovery: true})
	require.NoError(t, err)
	require.Len(t, selected.SubscriptionGroups, 1)
	require.Equal(t, []string{"gpt-5"}, FilterTeamModels(selected, []string{"gpt-5", "other"}))
	require.Empty(t, selected.Team.GroupIDs)
	require.Zero(t, repo.admitted)
	_, _, err = svc.SelectForRequest(context.Background(), key, SubscriptionRequest{Path: "/v1/responses", Models: []string{"gpt-5"}})
	require.ErrorIs(t, err, ErrNoEligibleSubscription)
	require.Zero(t, repo.admitted, "discovery never silently enables request routing")
}
