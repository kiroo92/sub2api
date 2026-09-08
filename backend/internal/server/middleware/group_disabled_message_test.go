//go:build unit

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthDisabledMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, google := range []bool{false, true} {
		for _, tc := range []struct {
			name, status, message, want string
			statusCode                  int
		}{
			{"custom", "inactive", "  该分组已停用，请切换至 XXX 分组使用  ", "该分组已停用，请切换至 XXX 分组使用", 403},
			{"empty", "inactive", "", "API Key 所属分组已停用", 403},
			{"whitespace", "inactive", " \n\t ", "API Key 所属分组已停用", 403},
			{"deleted", "deleted", "custom", "API Key 所属分组已删除", 403},
			{"active", "active", "custom", "", 204},
		} {
			name := tc.name
			if google {
				name += "/google"
			}
			t.Run(name, func(t *testing.T) {
				id := int64(1)
				key := &service.APIKey{ID: 1, Key: "test-key", Status: service.StatusActive, GroupID: &id,
					User:  &service.User{ID: 1, Status: service.StatusActive, Balance: 10},
					Group: &service.Group{ID: id, Status: tc.status, Platform: service.PlatformOpenAI, DisabledMessage: tc.message, Hydrated: true},
				}
				repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { return key, nil }}
				cfg := &config.Config{RunMode: config.RunModeSimple}
				svc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg)
				router := gin.New()
				var reason IngressRejectReason
				router.Use(func(c *gin.Context) { c.Next(); reason, _ = GetIngressRejectReason(c) })
				if google {
					router.Use(APIKeyAuthGoogle(svc, cfg))
				} else {
					router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
				}
				router.GET("/t", func(c *gin.Context) { c.Status(http.StatusNoContent) })
				req := httptest.NewRequest(http.MethodGet, "/t", nil)
				req.Header.Set("x-api-key", key.Key)
				req.Header.Set("x-goog-api-key", key.Key)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				require.Equal(t, tc.statusCode, w.Code, w.Body.String())
				if tc.want != "" {
					var body struct {
						Message string `json:"message"`
						Code    string `json:"code"`
						Error   struct {
							Message string `json:"message"`
						}
					}
					require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
					if google {
						require.Equal(t, tc.want, body.Error.Message)
					} else {
						require.Equal(t, tc.want, body.Message)
						if tc.status == "inactive" {
							require.Equal(t, "GROUP_DISABLED", body.Code)
						}
					}
				}
				if tc.status == "inactive" {
					require.Equal(t, IngressRejectGroupDisabled, reason)
				}
			})
		}
	}
}
