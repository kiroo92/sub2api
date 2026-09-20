package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
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
func (h *PaymentHandler) SaveInvoiceConfig(c *gin.Context) {
	var cfg service.InvoiceConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "Invalid invoice configuration")
		return
	}
	result, err := h.configService.SaveInvoiceConfig(c.Request.Context(), cfg)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
func (h *PaymentHandler) ListInvoices(c *gin.Context) {
	page, size := response.ParsePagination(c)
	uid, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	result, total, err := h.paymentService.ListInvoices(c.Request.Context(), service.InvoiceListParams{Page: page, PageSize: size, Status: c.Query("status"), Search: c.Query("search"), UserID: uid, StartDate: c.Query("start_date"), EndDate: c.Query("end_date")})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, result, int64(total), page, size)
}
func (h *PaymentHandler) GetInvoice(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	result, err := h.paymentService.GetInvoice(c.Request.Context(), 0, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
func (h *PaymentHandler) MarkInvoicesIssued(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin authentication required")
		return
	}
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid invoice selection")
		return
	}
	if err := h.paymentService.MarkInvoicesIssued(c.Request.Context(), subject.UserID, req.IDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}
