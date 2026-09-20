package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

func parseDiscountCodeFilter(c *gin.Context) int64 {
	id, _ := strconv.ParseInt(c.Query("discount_code_id"), 10, 64)
	return id
}

func (h *PaymentHandler) ListDiscountCodes(c *gin.Context) {
	page, size := response.ParsePagination(c)
	codes, total, err := h.configService.ListDiscountCodes(c.Request.Context(), page, size, c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, codes, int64(total), page, size)
}

func (h *PaymentHandler) CreateDiscountCode(c *gin.Context) { h.saveDiscountCode(c, 0) }

func (h *PaymentHandler) UpdateDiscountCode(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if ok {
		h.saveDiscountCode(c, id)
	}
}

func (h *PaymentHandler) saveDiscountCode(c *gin.Context, id int64) {
	req := service.SaveDiscountCodeRequest{Enabled: true, PerUserLimit: 1}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	code, err := h.configService.SaveDiscountCode(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if id == 0 {
		response.Created(c, code)
	} else {
		response.Success(c, code)
	}
}
