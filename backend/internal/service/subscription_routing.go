package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrNoEligibleSubscription = infraerrors.Forbidden("SUBSCRIPTION_NOT_AVAILABLE", "no eligible subscription is available")

type SubscriptionRequest struct {
	Body               []byte
	Models             []string
	Path               string
	Discovery          bool
	WebSocket          bool
	RequireSchedulable bool
}

// SelectForRequest reads individual records, never the single-row group cache.
func (s *SubscriptionService) SelectForRequest(ctx context.Context, key *APIKey, request SubscriptionRequest) (*APIKey, *UserSubscription, error) {
	if key.UsesTeam() {
		if err := ValidateTeamTextRequest(request.Path, request.Models, request.Body); err != nil {
			return nil, nil, err
		}
		selected, sub, err := s.selectTeamForRequest(ctx, key, request)
		if err == nil && !request.Discovery {
			err = s.AdmitTeamRequest(ctx, selected)
		}
		return selected, sub, err
	}
	return s.selectSubscriptionsForRequest(ctx, key, request, nil)
}

func (s *SubscriptionService) selectSubscriptionsForRequest(ctx context.Context, key *APIKey, request SubscriptionRequest, groupID *int64) (*APIKey, *UserSubscription, error) {
	if key == nil || key.User == nil || !key.UsesAllSubscriptions() {
		return nil, nil, ErrNoEligibleSubscription
	}
	subs, err := s.userSubRepo.ListActiveByUserID(ctx, key.User.ID)
	if err != nil {
		return nil, nil, err
	}
	selected := *key
	owner := *key.User
	owner.UserGroupRPMOverride = nil
	selected.User = &owner
	selected.Group, selected.GroupID, selected.SubscriptionGroups = nil, nil, nil
	selected.SubscriptionRoute = nil
	selected.SubscriptionRate = nil
	selected.SubscriptionPricingAt = time.Time{}
	now := time.Now()
	for i := range subs {
		sub := &subs[i]
		if groupID != nil && sub.GroupID != *groupID {
			continue
		}
		if sub.UserID != key.User.ID || sub.Status != SubscriptionStatusActive || sub.DeletedAt != nil || !sub.ExpiresAt.After(now) || sub.StartsAt.After(now) || sub.Group == nil || !sub.Group.IsActive() || !sub.Group.IsSubscriptionType() {
			continue
		}
		if request.Discovery {
			selected.SubscriptionGroups = append(selected.SubscriptionGroups, sub.Group)
			continue
		}
		compatible, err := s.subscriptionSupportsRequest(ctx, sub.Group, request)
		if err != nil {
			return nil, nil, err
		}
		if !compatible {
			continue
		}
		group := *sub.Group
		preview := *sub
		needsMaintenance, limitErr := s.ValidateAndCheckLimits(&preview, &group)
		if limitErr != nil || checkSelectedSubscription(&preview, key.User.ID, &group, now) != nil {
			continue
		}
		if needsMaintenance {
			sub, err = s.EnsureWindowMaintenance(ctx, sub)
			if err != nil {
				return nil, nil, err
			}
			if checkSelectedSubscription(sub, key.User.ID, &group, time.Now()) != nil {
				continue
			}
		}
		// An all-subscriptions key must never enter a fixed-group fallback path.
		group.FallbackGroupID, group.FallbackGroupIDOnInvalidRequest = nil, nil
		selected.GroupID, selected.Group = &sub.GroupID, &group
		rate := group.RateMultiplier
		if s.subscriptionRates != nil {
			userRate, err := s.subscriptionRates.GetByUserAndGroup(ctx, key.User.ID, group.ID)
			if err != nil {
				return nil, nil, err
			}
			if userRate != nil {
				rate = *userRate
			}
		}
		selected.SubscriptionRate, selected.SubscriptionPricingAt = &rate, time.Now()
		if request.WebSocket && group.Platform == PlatformComposite && len(request.Models) > 0 {
			decision, err := s.compositeResolver.Resolve(ctx, group.ID, request.Models[0], CompositeRouteEndpointResponses)
			if err != nil {
				return nil, nil, err
			}
			selected.SubscriptionRoute = &decision
		}
		return &selected, sub, nil
	}
	if request.Discovery {
		return &selected, nil, nil
	}
	return nil, nil, ErrNoEligibleSubscription
}

func SubscriptionRequestContext(ctx context.Context, key *APIKey) context.Context {
	if key.UsesTeam() {
		ctx = context.WithValue(ctx, ctxkey.TeamBilling, true)
	}
	ctx = context.WithValue(ctx, ctxkey.Group, key.Group)
	for _, name := range []ctxkey.Key{ctxkey.ResolvedTargetPlatform, ctxkey.ResolvedUpstreamModel, ctxkey.RequestedPublicModel, ctxkey.CompositeRouteSource} {
		ctx = context.WithValue(ctx, name, "")
	}
	if key.SubscriptionRoute != nil {
		ctx = WithCompositeRouteDecision(ctx, *key.SubscriptionRoute)
	}
	return context.WithValue(ctx, ctxkey.AllSubscriptions, true)
}

func (s *SubscriptionService) subscriptionSupportsRequest(ctx context.Context, group *Group, request SubscriptionRequest) (bool, error) {
	platform := group.Platform
	endpoint := CompositeRouteEndpointAny
	switch {
	case strings.Contains(request.Path, "/messages/count_tokens"):
		endpoint = CompositeRouteEndpointCountTokens
	case strings.Contains(request.Path, "/messages"):
		endpoint = CompositeRouteEndpointMessages
	case strings.Contains(request.Path, "/responses"):
		endpoint = CompositeRouteEndpointResponses
	case strings.Contains(request.Path, "/chat/completions"):
		endpoint = CompositeRouteEndpointChatCompletions
	case strings.Contains(request.Path, "/embeddings"):
		endpoint = CompositeRouteEndpointEmbeddings
	case strings.Contains(request.Path, "/images/"):
		endpoint = CompositeRouteEndpointImages
	case strings.Contains(request.Path, "/v1beta/"):
		endpoint = CompositeRouteEndpointGemini
	}
	models := request.Models
	if len(models) == 0 {
		models = []string{""}
	}
	for _, publicModel := range models {
		if !group.ModelAllowlist.Allows(publicModel) {
			return false, nil
		}
		model := publicModel
		modelCtx := SubscriptionRequestContext(ctx, &APIKey{Group: group})
		if group.Platform == PlatformComposite && model != "" {
			decision, err := s.compositeResolver.Resolve(ctx, group.ID, model, endpoint)
			if err != nil {
				return false, err
			}
			if !decision.Matched {
				return false, nil
			}
			platform, model = decision.TargetPlatform, decision.UpstreamModel
			modelCtx = WithCompositeRouteDecision(modelCtx, decision)
		}
		if strings.HasPrefix(request.Path, "/antigravity/") && platform != PlatformAntigravity {
			return false, nil
		}
		if request.WebSocket && platform != PlatformOpenAI && platform != PlatformGrok && platform != PlatformComposite {
			return false, nil
		}
		switch endpoint {
		case CompositeRouteEndpointGemini:
			if platform != PlatformGemini && platform != PlatformAntigravity {
				return false, nil
			}
		case CompositeRouteEndpointImages:
			if (platform != PlatformOpenAI && platform != PlatformGrok) || !GroupAllowsImageGeneration(group) {
				return false, nil
			}
		case CompositeRouteEndpointEmbeddings:
			if platform != PlatformOpenAI {
				return false, nil
			}
		case CompositeRouteEndpointMessages, CompositeRouteEndpointCountTokens:
			if platform == PlatformOpenAI && !group.AllowMessagesDispatch {
				return false, nil
			}
		}
		if strings.Contains(request.Path, "/videos") && platform != PlatformGrok {
			return false, nil
		}
		if strings.Contains(request.Path, "/live") || strings.Contains(request.Path, "/realtime/calls") {
			if platform != PlatformOpenAI || !group.AllowLive {
				return false, nil
			}
		}
		if s.channels != nil && model != "" {
			mapping, _ := s.channels.ResolveChannelMappingAndRestrict(ctx, &group.ID, model)
			model = mapping.MappedModel
		}
		if s.accountRepo != nil && model != "" {
			platforms := []string{platform}
			if platform == PlatformAnthropic || platform == PlatformGemini {
				platforms = append(platforms, PlatformAntigravity)
			}
			accounts, err := s.accountRepo.ListModelAvailabilityCandidates(ctx, &group.ID, platforms, false)
			if err != nil {
				return false, err
			}
			supported := false
			for i := range accounts {
				account := &accounts[i]
				if request.RequireSchedulable && !account.IsSchedulable() {
					continue
				}
				if account.Platform == PlatformAntigravity && platform != PlatformAntigravity && !account.IsMixedSchedulingEnabled() {
					continue
				}
				if known, ok := DetectModelPlatform(model); ok && known != platform && platform != PlatformAntigravity && len(account.GetModelMapping()) == 0 {
					continue
				}
				if (*GatewayService)(nil).isModelSupportedByAccountWithContext(modelCtx, account, model) {
					supported = true
					break
				}
			}
			if !supported {
				return false, nil
			}
		}
	}
	return true, nil
}
