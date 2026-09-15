package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	APIKeyRoutingFixedGroup  = "fixed_group"
	APIKeyRoutingAllPackages = "all_packages"
)

var (
	ErrNoUsablePackage          = infraerrors.Forbidden("PACKAGE_UNAVAILABLE", "No compatible package has remaining quota")
	ErrPackageSelectionRequired = errors.New("package billing requires an admitted package and period")
)

// PackageSelection belongs to one admitted request/turn, not to the stored Key.
// A delayed completion always charges this period, even after expiry or reorder.
type PackageSelection struct {
	PackageID      int64
	PeriodID       int64
	UserID         int64
	GroupID        int64
	PeriodEndsAt   time.Time
	QuotaUSD       float64
	UsedUSD        float64
	RateMultiplier float64
	PricingAt      time.Time
	CompositeRoute *CompositeRouteDecision
}

func (k *APIKey) UsesPackages() bool { return k != nil && k.RoutingMode == APIKeyRoutingAllPackages }

func normalizedAPIKeyRoutingMode(mode string) string {
	if mode == "" {
		return APIKeyRoutingFixedGroup
	}
	return mode
}

func validateAPIKeyRoutingMode(mode string, groupID *int64) error {
	mode = normalizedAPIKeyRoutingMode(mode)
	if mode != APIKeyRoutingFixedGroup && mode != APIKeyRoutingAllPackages {
		return infraerrors.BadRequest("API_KEY_ROUTING_MODE_INVALID", "Unknown API key routing mode")
	}
	if mode == APIKeyRoutingAllPackages && groupID != nil {
		return infraerrors.BadRequest("API_KEY_GROUP_CONFLICT", "Package keys cannot bind a fixed group")
	}
	return nil
}

func (s *APIKeyService) SetPackageService(packages *PackageService) { s.packageService = packages }

func (s *APIKeyService) ListKeyPackages(ctx context.Context, key *APIKey) ([]UserPackage, error) {
	if !key.UsesPackages() || s.packageService == nil {
		return nil, ErrPackageSelectionRequired
	}
	return s.packageService.ListOwned(ctx, key.UserID)
}

type PackageRequest struct {
	Models         []string
	Path           string
	ForcedPlatform string
	WebSocket      bool
}

// Wire this instead of NewChannelService: channel construction already depends
// on the Key invalidator, so the optional selection lookup introduces no cycle.
func ProvidePackageChannelService(repo ChannelRepository, groups GroupRepository, invalidator APIKeyAuthCacheInvalidator, pricing *PricingService) *ChannelService {
	channels := NewChannelService(repo, groups, invalidator, pricing)
	if keys, ok := invalidator.(*APIKeyService); ok {
		keys.SetPackageChannelService(channels)
	}
	return channels
}

func (s *APIKeyService) SetPackageChannelService(channels *ChannelService) {
	s.packageChannels = channels
}

// SelectPackage inspects candidates without charging RPM or touching old subscriptions.
func (s *APIKeyService) SelectPackage(ctx context.Context, key *APIKey, request PackageRequest) (*APIKey, error) {
	if key == nil || !key.UsesPackages() || key.User == nil {
		return nil, ErrPackageSelectionRequired
	}
	if s.packageService == nil || s.groupRepo == nil {
		return nil, ErrBillingServiceUnavailable
	}
	if err := s.recoverPendingPackageBilling(ctx, key.UserID); err != nil {
		return nil, err
	}
	packages, err := s.packageService.ListOwned(ctx, key.UserID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range packages {
		p := &packages[i]
		period := p.CurrentPeriod
		if p.UserID != key.UserID || p.Status != StatusActive || now.Before(p.StartsAt) || !now.Before(p.ExpiresAt) || period == nil || period.QuotaUSD <= period.UsedUSD || now.Before(period.StartsAt) || !now.Before(period.EndsAt) {
			continue
		}
		group, err := s.groupRepo.GetByID(ctx, p.GroupID)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				continue
			}
			return nil, err
		}
		if !packageGroupCompatible(group, request) {
			continue
		}
		compatible, route, err := s.packageAccountsCompatible(ctx, group, request)
		if err != nil {
			return nil, err
		}
		if !compatible {
			continue
		}
		copyKey, copyUser, copyGroup := *key, *key.User, *group
		// Package selection is the only entitlement router. A legacy fallback
		// group must not replace the paid group's permissions or attribution.
		copyGroup.FallbackGroupID, copyGroup.FallbackGroupIDOnInvalidRequest = nil, nil
		rate := group.RateMultiplier
		copyUser.UserGroupRPMOverride = nil
		if s.userGroupRateRepo != nil {
			override, err := s.userGroupRateRepo.GetRPMOverrideByUserAndGroup(ctx, key.UserID, group.ID)
			if err != nil {
				return nil, err
			}
			copyUser.UserGroupRPMOverride = override
			multiplier, err := s.userGroupRateRepo.GetByUserAndGroup(ctx, key.UserID, group.ID)
			if err != nil {
				return nil, err
			}
			if multiplier != nil {
				rate = *multiplier
			}
		}
		copyKey.User, copyKey.Group, copyKey.GroupID = &copyUser, &copyGroup, &copyGroup.ID
		copyKey.PackageSelection = &PackageSelection{PackageID: p.ID, PeriodID: period.ID, UserID: p.UserID, GroupID: p.GroupID, PeriodEndsAt: period.EndsAt, QuotaUSD: period.QuotaUSD, UsedUSD: period.UsedUSD, RateMultiplier: rate, PricingAt: now, CompositeRoute: route}
		return &copyKey, nil
	}
	return nil, ErrNoUsablePackage
}

// Pending records survive failed accounting transactions. Recover them before
// inspecting quota; otherwise stop admission instead of serving on stale usage.
func (s *APIKeyService) recoverPendingPackageBilling(ctx context.Context, userID int64) error {
	rows, err := s.packageService.client.QueryContext(ctx, `SELECT command FROM package_usage_billing WHERE user_id=$1 AND NOT applied ORDER BY id`, userID)
	if err != nil {
		return err
	}
	var commands []*UsageBillingCommand
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			_ = rows.Close()
			return err
		}
		var cmd UsageBillingCommand
		if err := json.Unmarshal(raw, &cmd); err != nil {
			_ = rows.Close()
			return err
		}
		commands = append(commands, &cmd)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, cmd := range commands {
		if s.packageBillingRepo == nil {
			return ErrBillingServiceUnavailable
		}
		if _, err := s.packageBillingRepo.Apply(ctx, cmd); err != nil {
			return err
		}
		// Any exhausted key must be reloaded even when a recovery, not its
		// original request, committed the quota increment.
		s.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if len(commands) > 0 {
		return ErrBillingServiceUnavailable
	} // retry with freshly reloaded key quota/status
	return nil
}

func (s *APIKeyService) SetPackageBillingRepository(repo UsageBillingRepository) {
	s.packageBillingRepo = repo
}

func (s *APIKeyService) SetPackageRoutingRepositories(accounts AccountRepository, resolver *CompositeRouteResolver) {
	s.packageAccountRepo, s.packageCompositeResolver = accounts, resolver
}

func (s *APIKeyService) packageAccountsCompatible(ctx context.Context, g *Group, req PackageRequest) (bool, *CompositeRouteDecision, error) {
	if s.packageAccountRepo == nil {
		return false, nil, ErrBillingServiceUnavailable
	}
	accounts, err := s.packageAccountRepo.ListSchedulableByGroupID(ctx, g.ID)
	if err != nil {
		return false, nil, err
	}
	models := req.Models
	if len(models) == 0 {
		models = []string{""}
	}
	var route *CompositeRouteDecision
	for i, model := range models {
		platform, upstream := g.Platform, model
		if platform == PlatformComposite {
			if s.packageCompositeResolver == nil {
				return false, nil, ErrBillingServiceUnavailable
			}
			decision, err := s.packageCompositeResolver.Resolve(ctx, g.ID, model, packageCompositeEndpoint(req.Path))
			if err != nil {
				return false, nil, err
			}
			if !decision.Matched {
				return false, nil, nil
			}
			if i == 0 {
				route = &decision
			}
			platform, upstream = decision.TargetPlatform, decision.UpstreamModel
		}
		if req.ForcedPlatform != "" {
			platform = req.ForcedPlatform
		}
		resolved := *g
		resolved.Platform = platform
		if !packageGroupCompatible(&resolved, req) {
			return false, nil, nil
		}
		if s.packageChannels != nil {
			lookup, err := s.packageChannels.lookupGroupChannel(ctx, g.ID)
			if err != nil {
				return false, nil, err
			}
			if lookup != nil {
				upstream = resolveMapping(lookup, g.ID, upstream).MappedModel
				if checkRestricted(lookup, g.ID, upstream) {
					return false, nil, nil
				}
			}
		}
		found := false
		for i := range accounts {
			a := &accounts[i]
			// Native voice models are not text model mappings. Match the existing
			// realtime scheduler's capability-only selection for these requests.
			modelSupported := upstream == "" || strings.HasSuffix(req.Path, "/realtime") || a.IsModelSupported(upstream)
			if a.Platform == platform && modelSupported && packageAccountSupportsEndpoint(a, req) && (!req.WebSocket || packageWSAccountTransportCompatible(a, s.cfg)) {
				found = true
				break
			}
		}
		if !found {
			return false, nil, nil
		}
	}
	return len(accounts) > 0, route, nil
}

func packageAccountSupportsEndpoint(a *Account, req PackageRequest) bool {
	path := req.Path
	if a.Platform != PlatformOpenAI && a.Platform != PlatformGrok {
		return true
	}
	capability := OpenAIEndpointCapabilityChatCompletions
	switch {
	case strings.Contains(path, "/embeddings"):
		capability = OpenAIEndpointCapabilityEmbeddings
	case strings.Contains(path, "/alpha/search"):
		capability = OpenAIEndpointCapabilityAlphaSearch
	case strings.Contains(path, "/realtime/calls"), strings.HasSuffix(path, "/live"):
		capability = OpenAIEndpointCapabilityLive
	case strings.Contains(path, "/images"), strings.Contains(path, "/videos"):
		if a.Platform == PlatformGrok {
			capability = OpenAIEndpointCapabilityGrokMediaGeneration
		} else {
			capability = OpenAIEndpointCapabilityResponses
		}
	}
	return a.SupportsOpenAIEndpointCapability(capability)
}

func packageCompositeEndpoint(path string) string {
	switch {
	case strings.Contains(path, "/messages/count_tokens"):
		return CompositeRouteEndpointCountTokens
	case strings.Contains(path, "/messages"):
		return CompositeRouteEndpointMessages
	case strings.Contains(path, "/responses"), strings.Contains(path, "/alpha/search"), strings.Contains(path, "/live"), strings.Contains(path, "/realtime/calls"):
		return CompositeRouteEndpointResponses
	case strings.Contains(path, "/chat/completions"):
		return CompositeRouteEndpointChatCompletions
	case strings.Contains(path, "/embeddings"):
		return CompositeRouteEndpointEmbeddings
	case strings.Contains(path, "/images/"):
		return CompositeRouteEndpointImages
	case strings.Contains(path, "/v1beta/"):
		return CompositeRouteEndpointGemini
	default:
		return CompositeRouteEndpointAny
	}
}

// PackageGroups includes exhausted-but-active holdings for model discovery.
func (s *APIKeyService) PackageGroups(ctx context.Context, key *APIKey) ([]*Group, error) {
	if key == nil || !key.UsesPackages() || s.packageService == nil {
		return nil, ErrPackageSelectionRequired
	}
	packages, err := s.packageService.ListOwned(ctx, key.UserID)
	if err != nil {
		return nil, err
	}
	groups := make([]*Group, 0)
	seen := make(map[int64]bool)
	now := time.Now()
	for _, p := range packages {
		if p.UserID != key.UserID || p.Status != StatusActive || now.Before(p.StartsAt) || !now.Before(p.ExpiresAt) || seen[p.GroupID] {
			continue
		}
		g, err := s.groupRepo.GetByID(ctx, p.GroupID)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				continue
			}
			return nil, err
		}
		seen[p.GroupID] = true
		if g.IsActive() {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

func packageGroupCompatible(g *Group, req PackageRequest) bool {
	if g == nil || !g.IsActive() {
		return false
	}
	platform := g.Platform
	if req.ForcedPlatform != "" && platform != req.ForcedPlatform && platform != PlatformComposite {
		return false
	}
	path := req.Path
	if req.WebSocket && platform != PlatformComposite {
		switch platform {
		case PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek:
		default:
			return false
		}
	}
	switch {
	case strings.Contains(path, "/realtime/calls"), strings.Contains(path, "/live"):
		if !g.AllowLive || (platform != PlatformOpenAI && platform != PlatformComposite) {
			return false
		}
	case strings.Contains(path, "/embeddings"), strings.Contains(path, "/alpha/search"):
		if platform != PlatformOpenAI && platform != PlatformComposite {
			return false
		}
	case strings.Contains(path, "/videos"), strings.Contains(path, "/realtime"), strings.Contains(path, "/audio/"), strings.HasSuffix(path, "/tts"), strings.HasSuffix(path, "/stt"), strings.Contains(path, "/custom-voices"), strings.Contains(path, "/x_search"), strings.HasSuffix(path, "/web_search"):
		if platform != PlatformGrok && platform != PlatformComposite {
			return false
		}
	case strings.Contains(path, "/images"):
		if !g.AllowImageGeneration || (platform != PlatformOpenAI && platform != PlatformGrok && platform != PlatformComposite) {
			return false
		}
	case strings.Contains(path, "/v1beta"):
		if platform != PlatformGemini && platform != PlatformAntigravity && platform != PlatformComposite {
			return false
		}
	}
	for _, model := range req.Models {
		if !g.ModelAllowlist.Allows(model) {
			return false
		}
	}
	return true
}

type packageRequestContextKey struct{}

func WithPackageRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, packageRequestContextKey{}, true)
}
func IsPackageRequest(ctx context.Context) bool {
	return ctx != nil && ctx.Value(packageRequestContextKey{}) == true
}

func (s *BillingCacheService) checkPackageBillingEligibility(ctx context.Context, user *User, key *APIKey, group *Group) error {
	if key.PackageJob != nil {
		return nil
	} // owner/key authenticated read, not new generation
	selected := key.PackageSelection
	if selected == nil || user == nil || group == nil || selected.UserID != user.ID || selected.GroupID != group.ID || selected.QuotaUSD <= selected.UsedUSD {
		return ErrPackageSelectionRequired
	}
	if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
		return ErrBillingServiceUnavailable
	}
	if key.IsExpired() {
		return ErrAPIKeyExpired
	}
	if key.IsQuotaExhausted() {
		return ErrAPIKeyQuotaExhausted
	}
	if key.HasRateLimits() {
		if err := s.checkAPIKeyRateLimits(ctx, key); err != nil {
			return err
		}
	}
	return s.checkRPM(ctx, user, group)
}
