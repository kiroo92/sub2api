package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type adminTeamRepoStub struct {
	service.TeamRepository
	called  bool
	owner   int64
	scopeOK bool
}

func (r *adminTeamRepoStub) AdminOwner(_ context.Context, id int64) (int64, error) {
	if id != 7 {
		return 0, service.ErrTeamForbidden
	}
	return 42, nil
}
func (r *adminTeamRepoStub) Update(ctx context.Context, id int64, _ service.TeamSettings) error {
	r.called = true
	r.owner = id
	r.scopeOK = service.TeamMatchesAdminScope(ctx, 7) && !service.TeamMatchesAdminScope(ctx, 8)
	return nil
}
func TestTeamAdminScopeUsesResolvedOwnerAndPreservesActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{service.RoleUser, service.RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			repo := &adminTeamRepoStub{}
			h := NewTeamHandler(service.NewTeamService(repo, nil, nil, nil, nil))
			router := gin.New()
			actor := int64(0)
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
				c.Set(string(middleware.ContextKeyUserRole), role)
				c.Next()
				s, _ := middleware.GetAuthSubjectFromContext(c)
				actor = s.UserID
			})
			router.PUT("/admin/teams/:team_id", h.AdminScope, h.Update)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/admin/teams/7?owner_id=123", strings.NewReader(`{"name":"Renamed","owner_id":123}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			require.Equal(t, int64(99), actor)
			if role == service.RoleAdmin {
				require.Equal(t, 200, w.Code)
				require.True(t, repo.called)
				require.Equal(t, int64(42), repo.owner)
				require.True(t, repo.scopeOK)
			} else {
				require.Equal(t, 403, w.Code)
				require.False(t, repo.called)
			}
		})
	}
}

func TestTeamAdminEndpointsRejectNonAdminWithoutTouchingServices(t *testing.T) {
	h := NewTeamHandler(nil)
	for _, fn := range []gin.HandlerFunc{h.AdminList, h.AdminConfig, h.AdminScope} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/admin/teams", nil)
		c.Set(string(middleware.ContextKeyUserRole), service.RoleUser)
		fn(c)
		require.Equal(t, 403, w.Code)
	}
}

func TestTeamModelDiscoveryOmitsImageGenerationOnlyForTeams(t *testing.T) {
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{7: {{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5": "gpt-5", "gpt-image-1": "gpt-image-1"}}}}}})
	groups := []*service.Group{{ID: 7, Platform: service.PlatformOpenAI}}
	ctx := context.Background()
	require.Contains(t, h.subscriptionModelIDs(ctx, groups), "gpt-image-1")
	models := h.subscriptionModelIDs(context.WithValue(ctx, ctxkey.TeamBilling, true), groups)
	require.Contains(t, models, "gpt-5")
	require.NotContains(t, models, "gpt-image-1")
}
