package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

// AvailableModels shares the gateway catalog, including group allowlists and
// composite routes, without applying the target member's existing restriction.
func (h *TeamHandler) AvailableModels(gateway *GatewayHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := teamSubject(c)
		if !ok {
			return
		}
		groups, err := h.service.ModelGroups(c.Request.Context(), id)
		if err != nil {
			teamResult(c, nil, err)
			return
		}
		ctx := context.WithValue(c.Request.Context(), ctxkey.TeamBilling, true)
		teamResult(c, gateway.subscriptionModelIDs(ctx, groups), nil)
	}
}
