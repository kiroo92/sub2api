package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterPackageRoutes(v1 *gin.RouterGroup, h *handler.PackageHandler, jwt middleware.JWTAuthMiddleware, admin middleware.AdminAuthMiddleware, audit middleware.AuditLogMiddleware, settings *service.SettingService, limits *middleware.PanelRateLimiter) {
	user := v1.Group("/packages", gin.HandlerFunc(jwt), middleware.BackendModeUserGuard(settings), limits.Global())
	user.GET("/plans", h.ListPlans)
	user.GET("/mine", h.Mine)
	user.PUT("/order", h.Reorder)
	user.GET("/groups", h.ListGroups)
	user.POST("/groups", h.CreateGroup)
	user.GET("/groups/:id", h.Group)
	admins := v1.Group("/admin/packages", gin.HandlerFunc(admin), gin.HandlerFunc(audit), middleware.AdminComplianceGuard(settings))
	admins.GET("/plans", h.AdminListPlans)
	admins.POST("/plans", h.SavePlan)
	admins.PUT("/plans/:id", h.SavePlan)
	admins.DELETE("/plans/:id", h.StopSale)
}
