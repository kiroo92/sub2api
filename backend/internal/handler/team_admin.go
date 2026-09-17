package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

const teamAdminOwnerContext = "team_admin_owner"

func teamPlatformAdmin(c *gin.Context) bool {
	if c.GetString(string(middleware.ContextKeyUserRole)) != service.RoleAdmin {
		response.Forbidden(c, "Administrator access required")
		c.Abort()
		return false
	}
	return true
}

// Bound only on administrator routes. Never replace the authenticated audit actor.
func (h *TeamHandler) AdminScope(c *gin.Context) {
	if !teamPlatformAdmin(c) {
		return
	}
	id, ok := teamParam(c, "team_id")
	if !ok {
		c.Abort()
		return
	}
	owner, err := h.service.AdminOwner(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		c.Abort()
		return
	}
	c.Set(teamAdminOwnerContext, owner)
	c.Request = c.Request.WithContext(service.WithTeamAdminScope(c.Request.Context(), id))
	c.Next()
}
func (h *TeamHandler) AdminList(c *gin.Context) {
	if !teamPlatformAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		response.BadRequest(c, "Invalid pagination")
		return
	}
	status := c.Query("status")
	if status != "" && status != "active" && status != "paused" {
		response.BadRequest(c, "Invalid status")
		return
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 200 {
		response.BadRequest(c, "Search is too long")
		return
	}
	items, total, err := h.service.AdminList(c.Request.Context(), search, status, page, size)
	teamResult(c, gin.H{"items": items, "total": total, "page": page, "page_size": size}, err)
}
func (h *TeamHandler) AdminConfig(c *gin.Context) {
	if !teamPlatformAdmin(c) {
		return
	}
	if c.Request.Method == "PUT" {
		var in struct {
			FrontendURL string `json:"frontend_url"`
		}
		if c.ShouldBindJSON(&in) != nil {
			response.BadRequest(c, "Invalid URL")
			return
		}
		if err := h.service.SetInvitationBaseURL(c.Request.Context(), in.FrontendURL); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	value, err := h.service.InvitationBaseURL(c.Request.Context())
	teamResult(c, gin.H{"frontend_url": value}, err)
}
