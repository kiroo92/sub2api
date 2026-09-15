package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PackageHandler struct{ packages *service.PackageService }

func NewPackageHandler(packages *service.PackageService) *PackageHandler {
	return &PackageHandler{packages: packages}
}

func (h *PackageHandler) ListPlans(c *gin.Context) {
	plans, err := h.packages.ListPlans(c.Request.Context(), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plans)
}
func (h *PackageHandler) AdminListPlans(c *gin.Context) {
	plans, err := h.packages.ListPlans(c.Request.Context(), true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, plans)
}
func (h *PackageHandler) SavePlan(c *gin.Context) {
	var p service.PackagePlan
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "invalid package plan")
		return
	}
	p.ID = 0
	if c.Param("id") != "" {
		id, ok := packageParamID(c)
		if !ok {
			return
		}
		p.ID = id
	}
	result, err := h.packages.SavePlan(c.Request.Context(), p)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
func (h *PackageHandler) StopSale(c *gin.Context) {
	id, ok := packageParamID(c)
	if !ok {
		return
	}
	if err := h.packages.StopSale(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *PackageHandler) Mine(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	list, err := h.packages.ListOwned(c.Request.Context(), user.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, list)
}
func (h *PackageHandler) Reorder(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	var req struct {
		PackageIDs []int64 `json:"package_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.PackageIDs) > 10000 {
		response.BadRequest(c, "invalid package order")
		return
	}
	if err := h.packages.Reorder(c.Request.Context(), user.UserID, req.PackageIDs); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *PackageHandler) ListGroups(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	list, err := h.packages.ListGroups(c.Request.Context(), user.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, list)
}
func (h *PackageHandler) Group(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	id, ok := packageParamID(c)
	if !ok {
		return
	}
	g, err := h.packages.GetGroup(c.Request.Context(), user.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, g)
}
func (h *PackageHandler) CreateGroup(c *gin.Context) {
	user, ok := requireAuth(c)
	if !ok {
		return
	}
	var req struct {
		PlanID int64 `json:"plan_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanID <= 0 {
		response.BadRequest(c, "invalid package plan")
		return
	}
	g, err := h.packages.CreateGroup(c.Request.Context(), user.UserID, req.PlanID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, g)
}
func packageParamID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid ID")
		return 0, false
	}
	return id, true
}
