package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type LuckyDrawHandler struct {
	service *service.LuckyDrawService
	cfg     *config.Config
}

func NewLuckyDrawHandler(s *service.LuckyDrawService, cfg *config.Config) *LuckyDrawHandler {
	return &LuckyDrawHandler{service: s, cfg: cfg}
}
func (h *LuckyDrawHandler) available(c *gin.Context) bool {
	if h.cfg != nil && h.cfg.RunMode == config.RunModeSimple {
		response.Forbidden(c, "简易模式不支持幸运抽奖")
		return false
	}
	return true
}
func (h *LuckyDrawHandler) Get(c *gin.Context) {
	if !h.available(c) {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	data, err := h.service.Snapshot(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
func (h *LuckyDrawHandler) Draw(c *gin.Context) {
	if !h.available(c) {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var input struct {
		RequestID     string `json:"request_id" binding:"required,uuid"`
		ConfigVersion int64  `json:"config_version" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "无效的抽奖请求")
		return
	}
	data, err := h.service.Draw(c.Request.Context(), subject.UserID, input.RequestID, input.ConfigVersion)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
func (h *LuckyDrawHandler) AdminGet(c *gin.Context) {
	if !h.available(c) {
		return
	}
	data, err := h.service.Snapshot(c.Request.Context(), 0)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
func (h *LuckyDrawHandler) Configure(c *gin.Context) {
	if !h.available(c) {
		return
	}
	var input service.LuckyDrawConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "配置格式错误")
		return
	}
	if err := h.service.Configure(c.Request.Context(), input); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	input.Version++
	response.Success(c, input)
}
