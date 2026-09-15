package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPackageAuthKeepsQuotaAndInfrastructureStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/v1/responses", "/v1/messages", "/v1beta/models/gemini:generateContent"} {
		for _, quota := range []bool{false, true} {
			key := &service.APIKey{ID: 1, UserID: 1, RoutingMode: service.APIKeyRoutingAllPackages, User: &service.User{ID: 1}}
			want := http.StatusServiceUnavailable
			if quota {
				key.Quota, key.QuotaUsed = 1, 1
				want = http.StatusTooManyRequests
			}
			keys := service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.POST(path, func(c *gin.Context) { authenticatePackageRequest(c, keys, key) })
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-5.1"}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			require.Equal(t, want, w.Code, "%s: %s", path, w.Body.String())
			require.NotContains(t, w.Body.String(), "metadata=", "internal error formatting must not escape into protocol responses")
		}
	}
}
