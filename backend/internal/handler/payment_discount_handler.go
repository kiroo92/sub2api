package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *PaymentHandler) QuoteSubscription(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	var req struct {
		PlanID      int64  `json:"plan_id" binding:"required,gt=0"`
		PaymentType string `json:"payment_type" binding:"required,max=30"`
		CouponCode  string `json:"coupon_code" binding:"max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	quote, err := h.paymentService.QuoteSubscription(c.Request.Context(), service.CreateOrderRequest{UserID: subject.UserID, PlanID: req.PlanID, PaymentType: req.PaymentType, CouponCode: req.CouponCode})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, quote)
}
