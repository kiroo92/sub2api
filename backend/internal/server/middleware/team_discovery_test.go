package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type teamDiscoverySettings struct{ service.SettingRepository }

func (teamDiscoverySettings) GetValue(context.Context, string) (string, error) {
	return "false", nil
}

func TestTeamKeyDiscoveryDoesNotRequireFixedGroupAssignment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := service.NewSettingService(teamDiscoverySettings{}, &config.Config{})
	for _, mode := range []string{"team", service.APIKeyRoutingAllSubscriptions, service.APIKeyRoutingFixedGroup} {
		t.Run(mode, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(ContextKeyAPIKey), &service.APIKey{RoutingMode: mode})
			})
			router.Use(RequireGroupAssignment(settings, AnthropicErrorWriter))
			router.GET("/v1/models", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"data": []string{"gpt-5"}}) })
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
			if mode == service.APIKeyRoutingFixedGroup {
				require.Equal(t, http.StatusForbidden, w.Code)
			} else {
				require.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}
