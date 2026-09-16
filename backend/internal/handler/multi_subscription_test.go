package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAllSubscriptionsUsagePersistsBeforeNextRequest(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	ctx := context.WithValue(context.Background(), ctxkey.AllSubscriptions, true)
	for _, submit := range []func(context.Context, service.UsageRecordTask){
		(&GatewayHandler{usageRecordWorkerPool: pool}).submitUsageRecordTask,
		(&GatewayHandler{usageRecordWorkerPool: pool}).submitMandatoryUsageRecordTask,
		(&OpenAIGatewayHandler{usageRecordWorkerPool: pool}).submitUsageRecordTask,
		(&OpenAIGatewayHandler{usageRecordWorkerPool: pool}).submitMandatoryUsageRecordTask,
	} {
		called := false
		submit(ctx, func(context.Context) { called = true })
		require.True(t, called, "all-subscription usage cannot wait in an asynchronous queue")
	}
}

func TestAllSubscriptionsAsyncImageKeepsAdmittedRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxkey.AllSubscriptions, true))
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", nil).WithContext(ctx)
	groupID := int64(3)
	key := &service.APIKey{ID: 2, UserID: 7, GroupID: &groupID, RoutingMode: service.APIKeyRoutingAllSubscriptions}
	sub := &service.UserSubscription{ID: 42, UserID: 7, GroupID: groupID}
	c.Set(string(middleware.ContextKeyAPIKey), key)
	c.Set(string(middleware.ContextKeySubscription), sub)
	task, _, stop := newAsyncImageContext(c, []byte(`{"model":"gpt-image-1"}`), time.Second)
	defer stop()
	cancel()
	c.Set(string(middleware.ContextKeySubscription), &service.UserSubscription{ID: 99})
	admitted, ok := middleware.GetSubscriptionFromContext(task)
	require.True(t, ok)
	require.Equal(t, int64(42), admitted.ID)
	require.NoError(t, task.Request.Context().Err())
	require.Equal(t, true, task.Request.Context().Value(ctxkey.AllSubscriptions))
}

func TestAllSubscriptionsRejectBalanceOnlyBatchBeforeSubmission(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/batches", nil)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{RoutingMode: service.APIKeyRoutingAllSubscriptions})
	(&BatchImageHandler{}).Submit(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
}
