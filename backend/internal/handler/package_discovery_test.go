package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type packageDiscoveryGroups struct{ service.GroupRepository }

func (packageDiscoveryGroups) GetByID(_ context.Context, id int64) (*service.Group, error) {
	models := []string{"gpt-a", "common"}
	if id == 2 {
		models = []string{"gpt-b", "common"}
	}
	return &service.Group{ID: id, Name: "package group", Status: service.StatusActive, Platform: service.PlatformOpenAI, RateMultiplier: float64(id), ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: models}}, nil
}

func TestPackageDiscoveryAndIntrospectionStayPackageOnly(t *testing.T) {
	db, client := packageWSDatabase(t)
	_, err := db.Exec(`UPDATE package_periods SET used_usd=quota_usd`)
	require.NoError(t, err)
	groups := packageDiscoveryGroups{}
	keys := service.NewAPIKeyService(nil, nil, groups, nil, nil, nil, nil)
	keys.SetPackageService(service.NewPackageService(client, groups))
	accounts := &gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		1: {{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-a": "gpt-a", "common": "gpt-a", "unpaid": "unpaid"}}}},
		2: {{ID: 2, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-b": "gpt-b", "common": "gpt-b"}}}},
		3: {{ID: 3, Platform: service.PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"old-only": "old-only"}}}},
	}}
	h := newGatewayModelsHandlerForTest(accounts)
	h.apiKeyService = keys
	h.openAIGatewayService = newKeyBillingOpenAIGatewayService(nil)
	key := &service.APIKey{ID: 1, UserID: 1, RoutingMode: service.APIKeyRoutingAllPackages, User: &service.User{ID: 1}}
	call := func(path, etag string, handler func(*gin.Context)) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, path, nil)
		c.Request.Header.Set("If-None-Match", etag)
		c.Set(string(middleware.ContextKeyAPIKey), key)
		handler(c)
		return w
	}
	response := call("/v1/models", "", h.Models)
	require.Equal(t, 200, response.Code)
	var models gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &models))
	require.Len(t, models.Data, 3)
	ids := []string{}
	for _, m := range models.Data {
		ids = append(ids, m.ID)
	}
	require.Contains(t, ids, "gpt-a")
	require.Contains(t, ids, "gpt-b")
	require.NotContains(t, ids, "old-only")
	require.NotContains(t, ids, "unpaid")
	etag := response.Header().Get("ETag")
	require.NotEmpty(t, etag)
	require.Equal(t, 304, call("/v1/models", etag, h.Models).Code)
	_, err = db.Exec(`UPDATE user_packages SET sort_order=3-id`)
	require.NoError(t, err)
	reordered := call("/v1/models", etag, h.Models)
	require.Equal(t, 200, reordered.Code)
	require.NotEqual(t, etag, reordered.Header().Get("ETag"))
	billing := call("/v1/sub2api/billing", "", h.KeyBillingInfo)
	require.Equal(t, 200, billing.Code)
	require.NotContains(t, billing.Body.String(), "balance")
	var info struct {
		Groups []packageGroupBillingInfo `json:"groups"`
	}
	require.NoError(t, json.Unmarshal(billing.Body.Bytes(), &info))
	require.Len(t, info.Groups, 2)
	require.Equal(t, int64(2), info.Groups[0].GroupID)
	require.Equal(t, 2.0, info.Groups[0].ResolvedRateMultiplier)
	usage := call("/v1/usage", "", h.Usage)
	require.Equal(t, 200, usage.Code)
	require.NotContains(t, usage.Body.String(), "balance")
	require.Contains(t, usage.Body.String(), `"type":"packages"`)
	require.Nil(t, key.GroupID)
	require.Nil(t, key.Group)
}
