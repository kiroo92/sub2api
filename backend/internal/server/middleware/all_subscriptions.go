package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func allSubscriptionsReadOnly(c *gin.Context) bool {
	return c.Request.Method == http.MethodGet && (strings.HasSuffix(c.FullPath(), "/models") || strings.HasSuffix(c.FullPath(), "/models/:model") || strings.HasSuffix(c.FullPath(), "/usage") || strings.HasSuffix(c.FullPath(), "/sub2api/billing") || allSubscriptionsResourceRead(c))
}

func allSubscriptionsResourceRead(c *gin.Context) bool {
	return c.Request.Method == http.MethodGet && (isAsyncImageTaskRead(c.Request.Method, c.Request.URL.Path) || (strings.Contains(c.FullPath(), "/videos/") && c.Param("request_id") != "") || c.Param("call_id") != "")
}

func selectAllSubscriptions(c *gin.Context, key *service.APIKey, subscriptions *service.SubscriptionService) (*service.APIKey, *service.UserSubscription, bool) {
	if key == nil || !key.UsesAllSubscriptions() {
		return key, nil, true
	}
	if allSubscriptionsResourceRead(c) {
		selected := *key
		selected.GroupID, selected.Group = nil, nil
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.AllSubscriptions, true))
		return &selected, nil, true
	}
	if subscriptions == nil {
		AbortWithError(c, http.StatusServiceUnavailable, "SUBSCRIPTION_SERVICE_UNAVAILABLE", "Subscription service is unavailable")
		return nil, nil, false
	}
	discovery := allSubscriptionsReadOnly(c) || isResponsesWebSocketRoute(c)
	var models []string
	if !discovery {
		if model := groupModelAllowlistModelFromParams(c); model != "" {
			models = []string{model}
		} else if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
			var ok bool
			models, ok = groupModelAllowlistModelsFromBody(c)
			if !ok {
				return nil, nil, false
			}
		}
	}

	if len(models) == 0 && c.Query("model") != "" {
		models = []string{c.Query("model")}
	}
	selected, sub, err := subscriptions.SelectForRequest(c.Request.Context(), key, service.SubscriptionRequest{
		Models: models, Path: c.Request.URL.Path, Discovery: discovery, WebSocket: isResponsesWebSocketRoute(c),
	})
	if err != nil {
		groupModelAllowlistErrorWriter(c)(c, infraerrors.Code(err), infraerrors.Message(err))
		c.Abort()
		return nil, nil, false
	}
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.AllSubscriptions, true))
	return selected, sub, true
}
