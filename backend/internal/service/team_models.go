package service

import (
	"context"
	"strings"
	"unicode"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrTeamModelForbidden = infraerrors.Forbidden("TEAM_MODEL_FORBIDDEN", "model is not allowed for this team member")

// ModelGroups uses configured groups, or previews the owner's available groups
// before routing is configured so member permissions can be set in advance.
func (s *TeamService) ModelGroups(ctx context.Context, owner int64) ([]*Group, error) {
	snapshot, err := s.repo.Snapshot(ctx, owner)
	if err != nil {
		return nil, err
	}
	if snapshot.Team == nil || snapshot.Role != "owner" {
		return nil, ErrTeamForbidden
	}
	available, err := s.keys.GetAvailableGroups(ctx, owner)
	if err != nil {
		return nil, err
	}
	groups := make([]*Group, 0)
	if len(snapshot.Team.GroupIDs) == 0 {
		for _, group := range available {
			if group.IsActive() {
				groups = append(groups, &group)
			}
		}
		return groups, nil
	}
	for _, id := range snapshot.Team.GroupIDs {
		for _, group := range available {
			if group.ID == id && group.IsActive() {
				groups = append(groups, &group)
				break
			}
		}
	}
	return groups, nil
}

func normalizeTeamModels(models []string) ([]string, error) {
	if len(models) > 200 {
		return nil, infraerrors.BadRequest("TEAM_MODELS_INVALID", "at most 200 model IDs are allowed")
	}
	out := make([]string, 0, len(models))
	seen := make(map[string]bool)
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || len(model) > 200 || strings.ContainsAny(model, "*?,") || strings.ContainsFunc(model, unicode.IsSpace) || strings.ContainsFunc(model, unicode.IsControl) {
			return nil, infraerrors.BadRequest("TEAM_MODELS_INVALID", "use exact model IDs without whitespace or wildcards")
		}
		if !seen[model] {
			seen[model] = true
			out = append(out, model)
		}
	}
	return out, nil
}

func (s *TeamService) SetModels(ctx context.Context, owner, member int64, models []string) error {
	models, err := normalizeTeamModels(models)
	if err != nil {
		return err
	}
	return s.repo.SetModels(ctx, owner, member, models)
}

// ValidateTeamModels matches public request IDs before any upstream model mapping.
func ValidateTeamModels(allowed, requested []string, discovery bool) error {
	if len(allowed) == 0 {
		return nil
	}
	if len(requested) == 0 && !discovery {
		return ErrTeamModelForbidden
	}
	for _, model := range requested {
		found := false
		for _, id := range allowed {
			if model == id {
				found = true
				break
			}
		}
		if !found {
			return ErrTeamModelForbidden
		}
	}
	return nil
}

func FilterTeamModels(key *APIKey, models []string) []string {
	if !key.UsesTeam() || key.Team == nil || len(key.Team.AllowedModels) == 0 {
		return models
	}
	out := make([]string, 0, len(models))
	for _, model := range models {
		if ValidateTeamModels(key.Team.AllowedModels, []string{model}, false) == nil {
			out = append(out, model)
		}
	}
	return out
}
