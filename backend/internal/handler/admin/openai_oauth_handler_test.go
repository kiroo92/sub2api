package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	openaipkg "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthHandlerCreateAccountFromAccessToken_OpenAI(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := NewOpenAIOAuthHandler(service.NewOpenAIOAuthService(nil, nil), adminSvc)

	router := gin.New()
	route := "/api/v1/admin/openai/create-from-access-token"
	router.POST(route, handler.CreateAccountFromAccessToken)

	payload, err := json.Marshal(map[string]any{
		"access_token": "at-openai-123",
		"email":        "openai@example.com",
		"concurrency":  2,
		"priority":     3,
		"group_ids":    []int64{11, 12},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 1)

	input := adminSvc.createdAccounts[0]
	require.Equal(t, "openai@example.com", input.Name)
	require.Equal(t, service.PlatformOpenAI, input.Platform)
	require.Equal(t, service.AccountTypeOAuth, input.Type)
	require.Equal(t, 2, input.Concurrency)
	require.Equal(t, 3, input.Priority)
	require.Equal(t, []int64{11, 12}, input.GroupIDs)
	require.Equal(t, "at-openai-123", input.Credentials["access_token"])
	require.Equal(t, openaipkg.ClientID, input.Credentials["client_id"])
	_, hasRefreshToken := input.Credentials["refresh_token"]
	require.False(t, hasRefreshToken)

	expiresAtRaw, ok := input.Credentials["expires_at"].(string)
	require.True(t, ok)
	expiresAt, err := time.Parse(time.RFC3339, expiresAtRaw)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(time.Hour), expiresAt, 5*time.Second)
}

func TestOpenAIOAuthHandlerCreateAccountFromAccessToken_SoraAcceptsAliasAndExplicitExpiry(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := NewOpenAIOAuthHandler(service.NewOpenAIOAuthService(nil, nil), adminSvc)

	router := gin.New()
	route := "/api/v1/admin/sora/create-from-access-token"
	router.POST(route, handler.CreateAccountFromAccessToken)

	expiresAtUnix := time.Now().Add(2 * time.Hour).Unix()
	payload, err := json.Marshal(map[string]any{
		"at":         "at-sora-456",
		"name":       "sora-direct-account",
		"expires_at": expiresAtUnix,
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 1)

	input := adminSvc.createdAccounts[0]
	require.Equal(t, "sora-direct-account", input.Name)
	require.Equal(t, service.PlatformSora, input.Platform)
	require.Equal(t, service.AccountTypeOAuth, input.Type)
	require.Equal(t, "at-sora-456", input.Credentials["access_token"])
	require.Equal(t, time.Unix(expiresAtUnix, 0).Format(time.RFC3339), input.Credentials["expires_at"])
}

func TestOpenAIOAuthHandlerCreateAccountFromAccessToken_RejectsPastExpiry(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := NewOpenAIOAuthHandler(service.NewOpenAIOAuthService(nil, nil), adminSvc)

	router := gin.New()
	route := "/api/v1/admin/openai/create-from-access-token"
	router.POST(route, handler.CreateAccountFromAccessToken)

	payload, err := json.Marshal(map[string]any{
		"access_token": "at-openai-123",
		"expires_at":   time.Now().Add(-time.Minute).Unix(),
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, adminSvc.createdAccounts)
}
