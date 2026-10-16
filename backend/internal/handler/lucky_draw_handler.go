package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
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
		Count         int    `json:"count"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "无效的抽奖请求")
		return
	}
	if input.Count == 0 {
		input.Count = 1
	}
	if input.Count != 1 && input.Count != 5 && input.Count != 10 {
		response.BadRequest(c, "请选择 1、5 或 10 次抽奖")
		return
	}
	// Keep the existing single-draw response compatible with older clients.
	if input.Count == 1 {
		data, err := h.service.Draw(c.Request.Context(), subject.UserID, input.RequestID, input.ConfigVersion)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, data)
		return
	}
	data, err := h.service.DrawBatch(c.Request.Context(), subject.UserID, input.RequestID, input.ConfigVersion, input.Count)
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
	// Start time is server-owned and cannot be changed by a config request.
	current, err := h.service.Snapshot(c.Request.Context(), 0)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, current.Config)
}
func (h *LuckyDrawHandler) Stats(c *gin.Context) {
	if !h.available(c) {
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 1000000 {
		response.BadRequest(c, "无效的页码")
		return
	}
	data, err := h.service.AdminStats(c.Request.Context(), c.Query("search"), page)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
func (h *LuckyDrawHandler) Grant(c *gin.Context) {
	if !h.available(c) {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var input struct {
		UserID    int64  `json:"user_id" binding:"required,gt=0"`
		RequestID string `json:"request_id" binding:"required,uuid"`
		Count     int    `json:"count" binding:"required,min=1,max=10000"`
		Note      string `json:"note"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请检查用户和赠送次数")
		return
	}
	data, err := h.service.GrantChances(c.Request.Context(), subject.UserID, input.UserID, input.RequestID, input.Count, input.Note)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
