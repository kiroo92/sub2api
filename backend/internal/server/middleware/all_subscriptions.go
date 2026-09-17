package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
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
	if key == nil || !key.UsesDynamicRouting() {
		return key, nil, true
	}
	if key.UsesTeam() {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.TeamBilling, true))
		if err := service.ValidateTeamTextRequest(c.Request.URL.Path, nil, nil); err != nil {
			AbortWithError(c, http.StatusBadRequest, "TEAM_ENDPOINT_UNSUPPORTED", err.Error())
			return nil, nil, false
		}
		var err error
		key, err = subscriptions.PrepareTeamKey(c.Request.Context(), key)
		if err != nil {
			groupModelAllowlistErrorWriter(c)(c, infraerrors.Code(err), infraerrors.Message(err))
			c.Abort()
			return nil, nil, false
		}
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
	var body []byte
	if key.UsesTeam() && !discovery && c.Request.Body != nil {
		var err error
		body, err = httputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			AbortWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Cannot read request body")
			return nil, nil, false
		}
		c.Request.Body = httputil.NewPrereadBody(body)
	}
	selected, sub, err := subscriptions.SelectForRequest(c.Request.Context(), key, service.SubscriptionRequest{
		Body:   body,
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

func releaseTeamRequest(c *gin.Context, subscriptions *service.SubscriptionService) {
	if c.GetBool("team_request_detached") {
		return
	}
	key, _ := GetAPIKeyFromContext(c)
	if !key.UsesTeam() || key.Team == nil || key.Team.RequestID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Second)
	defer cancel()
	if err := subscriptions.ReleaseTeamRequest(ctx, key); err != nil {
		slog.Error("release team request failed", "error", err)
	}
}
