package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

func (h *PaymentHandler) GetInvoiceConfig(c *gin.Context) {
	cfg, err := h.configService.GetInvoiceConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}
func (h *PaymentHandler) QuoteInvoice(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	var req service.InvoiceSelection
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid invoice selection")
		return
	}
	quote, err := h.paymentService.QuoteInvoice(c.Request.Context(), subject.UserID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, quote)
}
func (h *PaymentHandler) CreateInvoice(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	var req service.CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid invoice application")
		return
	}
	key := c.GetHeader("Idempotency-Key")
	executeUserIdempotentJSON(c, "payment.invoices.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.paymentService.CreateInvoice(ctx, subject.UserID, key, req)
	})
}
func (h *PaymentHandler) GetInvoice(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid invoice ID")
		return
	}
	invoice, err := h.paymentService.GetInvoice(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, invoice)
}

func (h *PaymentHandler) ListUnpaidInvoices(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	items, err := h.paymentService.ListUnpaidInvoices(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *PaymentHandler) CancelInvoice(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid invoice ID")
		return
	}
	result, err := h.paymentService.CancelInvoice(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
