package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type lotteryCaptchaVerifier interface {
	VerifyLotteryCaptcha(context.Context, service.CaptchaProof, string) error
}

type LotteryHandler struct {
	captcha lotteryCaptchaVerifier
	service *service.LotteryService
	cfg     *config.Config
}

func NewLotteryHandler(s *service.LotteryService, cfg *config.Config, auth *service.AuthService) *LotteryHandler {
	return &LotteryHandler{service: s, cfg: cfg, captcha: auth}
}
func (h *LotteryHandler) available(c *gin.Context) bool {
	if h.cfg != nil && h.cfg.RunMode == config.RunModeSimple {
		response.Forbidden(c, "简易模式不支持余额抽奖")
		return false
	}
	return true
}
func (h *LotteryHandler) Get(c *gin.Context) {
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
func (h *LotteryHandler) Join(c *gin.Context) {
	if !h.available(c) {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var input struct {
		RoundID        int64  `json:"round_id" binding:"required,gt=0"`
		TurnstileToken string `json:"turnstile_token" binding:"max=16384"`
		TencentTicket  string `json:"tencent_captcha_ticket" binding:"max=4096"`
		TencentRandstr string `json:"tencent_captcha_randstr" binding:"max=1024"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "请选择有效的抽奖期数")
		return
	}
	if h.captcha == nil {
		response.ErrorFrom(c, service.ErrServiceUnavailable)
		return
	}
	if err := h.captcha.VerifyLotteryCaptcha(c.Request.Context(), captchaProof(input.TurnstileToken, input.TencentTicket, input.TencentRandstr), ip.GetClientIP(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	data, err := h.service.Join(c.Request.Context(), subject.UserID, input.RoundID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}

// Admin methods are registered exclusively behind AdminAuthMiddleware.
func (h *LotteryHandler) AdminGet(c *gin.Context) {
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
func (h *LotteryHandler) Configure(c *gin.Context) {
	if !h.available(c) {
		return
	}
	var input service.LotteryConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "抽奖配置格式错误")
		return
	}
	if err := h.service.Configure(c.Request.Context(), input); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, input)
}
