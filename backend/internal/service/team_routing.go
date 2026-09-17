package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"time"
)

// PrepareTeamKey resolves fresh team state even when the credential came from cache.
// Personal keys never enter this path, including keys whose team has been deleted.
func (s *SubscriptionService) PrepareTeamKey(ctx context.Context, key *APIKey) (*APIKey, error) {
	if !key.UsesTeam() {
		return key, nil
	}
	if s == nil || s.teamRepo == nil || s.teamUsers == nil {
		return nil, ErrBillingServiceUnavailable
	}
	a, err := s.teamRepo.KeyAttribution(ctx, key.ID)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrTeamUnavailable
	}
	member, err := s.teamUsers.GetByID(ctx, a.MemberUserID)
	if err != nil {
		return nil, err
	}
	if !member.IsActive() {
		return nil, ErrTeamUnavailable
	}
	owner, err := s.teamUsers.GetByID(ctx, a.OwnerID)
	if err != nil {
		return nil, err
	}
	if !owner.IsActive() {
		return nil, ErrTeamUnavailable
	}
	selected := *key
	selected.Team = a
	if selected.TeamBillingUnrecorded == nil {
		selected.TeamBillingUnrecorded = &atomic.Bool{}
	}
	selected.User = owner
	selected.UserID = owner.ID
	if key.Team != nil {
		selected.Team.RequestID = key.Team.RequestID
	}
	return &selected, nil
}

func (s *SubscriptionService) selectTeamForRequest(ctx context.Context, key *APIKey, request SubscriptionRequest) (*APIKey, *UserSubscription, error) {
	request.RequireSchedulable = true
	key, err := s.PrepareTeamKey(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	result := *key
	result.SubscriptionGroups = nil
	for _, id := range key.Team.GroupIDs {
		group, err := s.groupRepo.GetByID(ctx, id)
		if errors.Is(err, ErrGroupNotFound) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if group == nil || !group.IsActive() {
			continue
		}
		if group.IsSubscriptionType() {
			// Reuse exact-subscription selection, constrained to this team's selected group.
			candidate := *key
			candidate.RoutingMode = APIKeyRoutingAllSubscriptions
			candidate.Team = nil
			selected, sub, err := s.selectSubscriptionsForRequest(ctx, &candidate, request, &id)
			if err == ErrNoEligibleSubscription {
				continue
			}
			if err != nil {
				return nil, nil, err
			}
			if request.Discovery {
				result.SubscriptionGroups = append(result.SubscriptionGroups, selected.SubscriptionGroups...)
				continue
			}
			selected.RoutingMode = "team"
			selected.Team = key.Team
			selected.Group.AllowImageGeneration = false
			return selected, sub, nil
		}
		if !key.User.CanBindGroup(group.ID, group.IsExclusive) {
			continue
		}
		if request.Discovery {
			result.SubscriptionGroups = append(result.SubscriptionGroups, group)
			continue
		}
		if key.User.Balance-key.User.FrozenBalance <= 0 {
			continue
		}
		compatible, err := s.subscriptionSupportsRequest(ctx, group, request)
		if err != nil {
			return nil, nil, err
		}
		if !compatible {
			continue
		}
		selected := *key
		g := *group
		g.AllowImageGeneration = false
		g.FallbackGroupID = nil
		g.FallbackGroupIDOnInvalidRequest = nil
		selected.Group = &g
		selected.GroupID = &g.ID
		selected.SubscriptionRate = nil
		selected.SubscriptionRoute = nil
		rate := g.RateMultiplier
		if s.subscriptionRates != nil {
			custom, err := s.subscriptionRates.GetByUserAndGroup(ctx, key.User.ID, g.ID)
			if err != nil {
				return nil, nil, err
			}
			if custom != nil {
				rate = *custom
			}
		}
		selected.SubscriptionRate = &rate
		selected.SubscriptionPricingAt = time.Now()
		if request.WebSocket && g.Platform == PlatformComposite && len(request.Models) > 0 {
			decision, err := s.compositeResolver.Resolve(ctx, g.ID, request.Models[0], CompositeRouteEndpointResponses)
			if err != nil {
				return nil, nil, err
			}
			selected.SubscriptionRoute = &decision
		}
		return &selected, nil, nil
	}
	if request.Discovery {
		return &result, nil, nil
	}
	return nil, nil, ErrNoEligibleSubscription
}

func (s *SubscriptionService) AdmitTeamRequest(ctx context.Context, key *APIKey) error {
	if !key.UsesTeam() {
		return nil
	}
	if key.Team == nil || s.teamRepo == nil {
		return ErrTeamUnavailable
	}
	if key.Team.RequestID == "" {
		var b [24]byte
		if _, err := rand.Read(b[:]); err != nil {
			return err
		}
		key.Team.RequestID = hex.EncodeToString(b[:])
	}
	return s.teamRepo.Admit(ctx, key.Team, key.ID)
}

func (s *SubscriptionService) ReleaseTeamRequest(ctx context.Context, key *APIKey) error {
	if !key.UsesTeam() || key.Team == nil || key.Team.RequestID == "" {
		return nil
	}
	unresolved := key.TeamBillingUnrecorded != nil && key.TeamBillingUnrecorded.Load()
	return s.teamRepo.CloseRequest(ctx, key.Team.RequestID, unresolved)
}
