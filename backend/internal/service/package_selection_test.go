package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPackageKeyModeAndGroupBoundary(t *testing.T) {
	gid := int64(4)
	for _, mode := range []string{"", APIKeyRoutingFixedGroup} {
		require.NoError(t, validateAPIKeyRoutingMode(mode, &gid))
	}
	require.NoError(t, validateAPIKeyRoutingMode(APIKeyRoutingAllPackages, nil))
	require.Error(t, validateAPIKeyRoutingMode(APIKeyRoutingAllPackages, &gid))
	require.Error(t, validateAPIKeyRoutingMode("all_subscriptions", nil))
	require.False(t, (&APIKey{GroupID: nil}).UsesPackages(), "legacy null groups must not become package keys")
}

type packageCompatibilityAccounts struct {
	AccountRepository
	accounts []Account
}

func (r packageCompatibilityAccounts) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return r.accounts, nil
}

type packageCompatibilityChannels struct {
	ChannelRepository
	fail bool
}

func (r packageCompatibilityChannels) ListAll(context.Context) ([]Channel, error) {
	if r.fail {
		return nil, errors.New("channel database unavailable")
	}
	return []Channel{{ID: 1, Status: StatusActive, GroupIDs: []int64{1}, ModelMapping: map[string]map[string]string{PlatformOpenAI: {"friendly": "gpt-5.1"}}}}, nil
}
func (packageCompatibilityChannels) GetGroupPlatforms(context.Context, []int64) (map[int64]string, error) {
	return map[int64]string{1: PlatformOpenAI}, nil
}

func TestPackageCompatibilityUsesChannelAndEndpoint(t *testing.T) {
	ctx := context.Background()
	g := &Group{ID: 1, Status: StatusActive, Platform: PlatformOpenAI}
	s := &APIKeyService{packageAccountRepo: packageCompatibilityAccounts{accounts: []Account{{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.1": "gpt-5.1"}}}}}}
	req := PackageRequest{Models: []string{"friendly"}, Path: "/v1/responses"}
	ok, _, err := s.packageAccountsCompatible(ctx, g, req)
	require.NoError(t, err)
	require.False(t, ok)
	s.SetPackageChannelService(NewChannelService(packageCompatibilityChannels{}, nil, nil, nil))
	ok, _, err = s.packageAccountsCompatible(ctx, g, req)
	require.NoError(t, err)
	require.True(t, ok, "published channel aliases must resolve before account support checks")
	s.SetPackageChannelService(NewChannelService(packageCompatibilityChannels{fail: true}, nil, nil, nil))
	_, _, err = s.packageAccountsCompatible(ctx, g, req)
	require.Error(t, err, "channel errors are not eligibility")
	s.packageChannels = nil
	s.packageAccountRepo = packageCompatibilityAccounts{accounts: []Account{{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}}}
	ok, _, err = s.packageAccountsCompatible(ctx, g, PackageRequest{Path: "/v1/embeddings", Models: []string{"text-embedding-3-small"}})
	require.NoError(t, err)
	require.False(t, ok, "OAuth lacks embeddings")
	g.AllowLive = true
	ok, _, err = s.packageAccountsCompatible(ctx, g, PackageRequest{Path: "/backend-api/codex/realtime/calls", Models: []string{"gpt-live"}})
	require.NoError(t, err)
	require.True(t, ok, "Codex live is OpenAI, not Grok realtime")
	g.Platform = PlatformAnthropic
	require.False(t, packageGroupCompatible(g, PackageRequest{Path: "/v1/responses", WebSocket: true}), "skip a non-WS platform instead of selecting a doomed card")
}

func TestPackageCompositeCompatibilityUsesResolvedEndpointPlatform(t *testing.T) {
	g := &Group{ID: 1, Status: StatusActive, Platform: PlatformComposite}
	s := &APIKeyService{packageAccountRepo: packageCompatibilityAccounts{accounts: []Account{{ID: 1, Platform: PlatformGrok, Type: AccountTypeAPIKey}}}, packageCompositeResolver: NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{{ID: 1, GroupID: 1, Enabled: true, PublicModel: "vector", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformGrok, UpstreamModel: "grok-4", Endpoint: CompositeRouteEndpointAny}}})}
	ok, _, err := s.packageAccountsCompatible(context.Background(), g, PackageRequest{Path: "/v1/embeddings", Models: []string{"vector"}})
	require.NoError(t, err)
	require.False(t, ok, "a composite alias targeting Grok cannot supply OpenAI embeddings")
}

func TestPackageNativeVoiceUsesEndpointCapability(t *testing.T) {
	s := &APIKeyService{packageAccountRepo: packageCompatibilityAccounts{accounts: []Account{{ID: 1, Platform: PlatformGrok, Type: AccountTypeAPIKey}}}}
	group := &Group{ID: 1, Status: StatusActive, Platform: PlatformGrok}
	for _, request := range []PackageRequest{
		{Path: "/v1/realtime"},
		{Path: "/v1/realtime", Models: []string{"grok-voice-latest"}},
		{Path: "/v1/tts"},
		{Path: "/v1/stt"},
	} {
		ok, _, err := s.packageAccountsCompatible(context.Background(), group, request)
		require.NoError(t, err)
		require.True(t, ok, "voice admission cannot require a text-model mapping for %s", request.Path)
	}
}

func TestPackageBillingNeverFallsBackToBalance(t *testing.T) {
	s := &BillingCacheService{}
	user := &User{ID: 1, Balance: 100}
	key := &APIKey{UserID: 1, RoutingMode: APIKeyRoutingAllPackages, User: user}
	err := s.CheckBillingEligibility(context.Background(), user, key, nil, nil, PlatformOpenAI)
	require.ErrorIs(t, err, ErrPackageSelectionRequired)
	group := &Group{ID: 2}
	key.PackageSelection = &PackageSelection{UserID: 1, GroupID: 3, QuotaUSD: 10, PeriodEndsAt: time.Now().Add(time.Hour)}
	require.ErrorIs(t, s.CheckBillingEligibility(context.Background(), user, key, group, nil, PlatformOpenAI), ErrPackageSelectionRequired)
}

func TestPackageUsageCommandFreezesIndependentIdentity(t *testing.T) {
	key := &APIKey{ID: 1, RoutingMode: APIKeyRoutingAllPackages, Quota: 100, PackageSelection: &PackageSelection{PackageID: 10, PeriodID: 11}}
	p := &postUsageBillingParams{APIKey: key, User: &User{ID: 2}, Account: &Account{ID: 3}, Cost: &CostBreakdown{TotalCost: 2, ActualCost: 3}}
	cmd := buildUsageBillingCommand("one", &UsageLog{}, p)
	require.Equal(t, 10, int(cmd.PackageID))
	require.Equal(t, 11, int(cmd.PackagePeriodID))
	require.Equal(t, 3.0, cmd.PackageCost)
	require.Zero(t, cmd.BalanceCost)
	require.Zero(t, cmd.SubscriptionCost)
	require.Nil(t, cmd.SubscriptionID)
	old := cmd.RequestFingerprint
	other := *cmd
	other.PackagePeriodID++
	other.RequestFingerprint = ""
	other.Normalize()
	require.NotEqual(t, old, other.RequestFingerprint)
	_, err := applyUsageBilling(context.Background(), "one", &UsageLog{}, p, &billingDeps{}, nil)
	require.ErrorIs(t, err, ErrPackageSelectionRequired, "missing atomic repository must not take legacy balance fallback")
}

func TestPackageRequestCompatibility(t *testing.T) {
	g := &Group{ID: 1, Status: StatusActive, Platform: PlatformOpenAI, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"gpt-*"}}}
	require.True(t, packageGroupCompatible(g, PackageRequest{Path: "/v1/responses", Models: []string{"gpt-5"}}))
	require.False(t, packageGroupCompatible(g, PackageRequest{Path: "/v1/responses", Models: []string{"gpt-5", "claude-sonnet"}}))
	require.False(t, packageGroupCompatible(g, PackageRequest{Path: "/v1beta/models/gemini:generateContent"}))
	require.False(t, packageGroupCompatible(g, PackageRequest{Path: "/v1/web_search"}), "the Grok-only endpoint must skip OpenAI packages")
	require.False(t, packageGroupCompatible(g, PackageRequest{Path: "/v1/images/generations", Models: []string{"gpt-image-1"}}))
	g.AllowImageGeneration = true
	require.True(t, packageGroupCompatible(g, PackageRequest{Path: "/v1/images/generations", Models: []string{"gpt-image-1"}}))
}
