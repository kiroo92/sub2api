package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type teamModelsRepoStub struct {
	service.TeamRepository
	owner, member int64
	models        []string
	called        bool
}

func (r *teamModelsRepoStub) SetModels(_ context.Context, owner, member int64, models []string) error {
	r.owner, r.member, r.models, r.called = owner, member, models, true
	return nil
}

func TestTeamModelsRequiresExplicitArrayAndTargetsIndividualMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{}`, 400},
		{`{"allowed_models":null}`, 400},
		{`{"allowed_models":"gpt-5"}`, 400},
		{`{"allowed_models":["gpt-*"]}`, 400},
		{`{"allowed_models":[]}`, 200},
		{`{"allowed_models":["gpt-5","gpt-5"]}`, 200},
	} {
		t.Run(tc.body, func(t *testing.T) {
			repo := &teamModelsRepoStub{}
			h := NewTeamHandler(service.NewTeamService(repo, nil, nil, nil, nil))
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
			})
			router.PUT("/team/members/:user_id/models", h.Models)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/team/members/8/models", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			require.Equal(t, tc.code, w.Code, w.Body.String())
			require.Equal(t, tc.code == 200, repo.called)
			if repo.called {
				require.Equal(t, int64(7), repo.owner)
				require.Equal(t, int64(8), repo.member)
				require.NotNil(t, repo.models)
				require.LessOrEqual(t, len(repo.models), 1)
			}
		})
	}
}
