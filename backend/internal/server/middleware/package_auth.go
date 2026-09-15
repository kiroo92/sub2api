package middleware

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// authenticatePackageRequest is a separate entitlement path after normal key,
// user and IP authentication. It never calls legacy subscription/balance checks.
func authenticatePackageRequest(c *gin.Context, keys *service.APIKeyService, key *service.APIKey) {
	writeError := groupModelAllowlistErrorWriter(c)
	path := c.Request.URL.Path
	if c.Request.Method == http.MethodPost && strings.HasSuffix(path, "/images/batches") {
		writeError(c, http.StatusBadRequest, "Package keys cannot create balance-funded batch jobs")
		c.Abort()
		return
	}
	discovery := c.Request.Method == http.MethodGet && (strings.HasSuffix(path, "/models") || strings.Contains(path, "/models/"))
	resourceRead := isAsyncImageTaskRead(c.Request.Method, path) || path == "/v1/usage" || path == "/v1/sub2api/billing"
	if strings.Contains(path, "/images/batches") {
		resourceRead = true
	}
	if c.Request.Method == http.MethodGet && (c.Param("request_id") != "" || c.Param("call_id") != "" || isAsyncImageTaskRead(c.Request.Method, path)) {
		kind, id := "grok_video", c.Param("request_id")
		if c.Param("call_id") != "" {
			kind, id = "live", c.Param("call_id")
		} else if isAsyncImageTaskRead(c.Request.Method, path) {
			kind, id = "image", c.Param("task_id")
		}
		restored, err := keys.RestorePackageJob(c.Request.Context(), key, kind, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(c, http.StatusNotFound, "Package task not found")
				c.Abort()
				return
			}
			writeError(c, 503, "Original package task attribution is unavailable")
			c.Abort()
			return
		}
		key = restored
		resourceRead = true
	}
	if !resourceRead {
		if err := keys.CheckAPIKeyQuotaAndExpiry(key); err != nil {
			writeError(c, infraerrors.Code(err), infraerrors.Message(err))
			c.Abort()
			return
		}
	}
	if !discovery && !resourceRead && !isResponsesWebSocketRoute(c) {
		request := service.PackageRequest{Path: path}
		if platform, ok := GetForcePlatformFromContext(c); ok {
			request.ForcedPlatform = platform
		}
		if model := groupModelAllowlistModelFromParams(c); model != "" {
			request.Models = []string{model}
		} else if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
			models, ok := groupModelAllowlistModelsFromBody(c)
			if !ok {
				return
			}
			request.Models = models
		}
		if len(request.Models) == 0 && c.Query("model") != "" {
			request.Models = []string{c.Query("model")}
		}
		selected, err := keys.SelectPackage(c.Request.Context(), key, request)
		if err != nil {
			writeError(c, infraerrors.Code(err), infraerrors.Message(err))
			c.Abort()
			return
		}
		key = selected
	}
	c.Set(string(ContextKeyAPIKey), key)
	c.Set(string(ContextKeyUser), AuthSubject{UserID: key.UserID, Concurrency: key.User.Concurrency})
	c.Set(string(ContextKeyUserRole), key.User.Role)
	setGroupContext(c, key.Group)
	c.Request = c.Request.WithContext(service.WithPackageRequest(context.WithValue(c.Request.Context(), ctxkey.UserID, key.UserID)))
	_ = keys.TouchLastUsed(c.Request.Context(), key.ID)
	c.Next()
}
