package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type orderedSubscriptionsRepo struct {
	service.UserSubscriptionRepository
	rows []service.UserSubscription
}

func (r *orderedSubscriptionsRepo) ListActiveByUserID(context.Context, int64) ([]service.UserSubscription, error) {
	return r.rows, nil
}

type allSubscriptionsKeyRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r *allSubscriptionsKeyRepo) GetByKeyForAuth(context.Context, string) (*service.APIKey, error) {
	clone := *r.key
	return &clone, nil
}
func (r *allSubscriptionsKeyRepo) UpdateLastUsed(context.Context, int64, time.Time) error { return nil }

func TestAllSubscriptionsAuthenticationNeverUsesBalanceAndRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	limit := 5.0
	group := &service.Group{ID: 8, Status: service.StatusActive, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeSubscription, DailyLimitUSD: &limit}
	first := service.UserSubscription{ID: 31, UserID: 7, GroupID: 8, Group: group, Status: service.SubscriptionStatusActive, StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), DailyUsageUSD: limit, DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now}
	second := first
	second.ID, second.DailyUsageUSD = 32, 0
	repo := &orderedSubscriptionsRepo{rows: []service.UserSubscription{first, second}}
	subs := service.NewSubscriptionService(nil, repo, nil, nil, nil)
	t.Cleanup(subs.Stop)
	key := &service.APIKey{ID: 1, UserID: 7, RoutingMode: service.APIKeyRoutingAllSubscriptions, Status: service.StatusActive, User: &service.User{ID: 7, Status: service.StatusActive, Balance: 0}}
	keys := service.NewAPIKeyService(&allSubscriptionsKeyRepo{key: key}, nil, nil, nil, nil, nil, &config.Config{})
	router := gin.New()
	router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(keys, subs, &config.Config{})))
	router.POST("/v1/responses", func(c *gin.Context) {
		sub, ok := GetSubscriptionFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(32), sub.ID)
		var body map[string]string
		require.NoError(t, c.ShouldBindJSON(&body))
		require.Equal(t, "gpt-5", body["model"])
		c.Status(http.StatusOK)
	})
	router.GET("/v1/images/tasks/:task_id", RequireGroupAssignment(nil, OpenAIErrorWriter), func(c *gin.Context) { c.Status(http.StatusOK) })
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer test-subscription-key-1234")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, http.StatusOK, request("POST", "/v1/responses", `{"model":"gpt-5"}`).Code)
	require.Nil(t, key.GroupID)
	repo.rows = nil
	key.User.Balance = 100
	require.Equal(t, http.StatusForbidden, request("POST", "/v1/responses", `{"model":"gpt-5"}`).Code)
	require.Equal(t, http.StatusOK, request("GET", "/v1/images/tasks/imgtask_owned", "").Code, "completed tasks remain readable after all subscriptions expire")
}
