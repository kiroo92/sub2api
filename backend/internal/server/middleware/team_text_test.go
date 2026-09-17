package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestTeamTextOnlyRejectsMediaBeforeAdmissionWithoutChangingPersonalKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/v1/images/generations", "/v1/images/edits/async", "/v1/videos", "/v1/live", "/v1/images/batches"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", path, nil)
		_, _, ok := selectAllSubscriptions(c, &service.APIKey{RoutingMode: "team"}, nil)
		require.False(t, ok)
		require.Equal(t, 400, w.Code)
		require.Contains(t, w.Body.String(), "TEAM_ENDPOINT_UNSUPPORTED")
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", path, nil)
		personal := &service.APIKey{RoutingMode: service.APIKeyRoutingFixedGroup}
		selected, _, ok := selectAllSubscriptions(c, personal, nil)
		require.True(t, ok)
		require.Same(t, personal, selected)
	}
}
