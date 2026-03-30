package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UserHandler handles user-related requests
type UserHandler struct {
	userService   *service.UserService
	inviteService *service.InviteService
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(userService *service.UserService, inviteService *service.InviteService) *UserHandler {
	return &UserHandler{
		userService:   userService,
		inviteService: inviteService,
	}
}

// ChangePasswordRequest represents the change password request payload
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// UpdateProfileRequest represents the update profile request payload
type UpdateProfileRequest struct {
	Username *string `json:"username"`
}

type BindInviteRequest struct {
	ReferralCode string `json:"referral_code" binding:"required"`
}

// GetProfile handles getting user profile
// GET /api/v1/users/me
func (h *UserHandler) GetProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	userData, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromService(userData))
}

// GetInvite handles getting current user invite info
func (h *UserHandler) GetInvite(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.inviteService == nil {
		response.InternalError(c, "Invite service unavailable")
		return
	}

	invite, err := h.inviteService.GetInviteInfoForUser(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, invite)
}

// BindInvite handles binding referral code for current user
func (h *UserHandler) BindInvite(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.inviteService == nil {
		response.InternalError(c, "Invite service unavailable")
		return
	}

	var req BindInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	binding, err := h.inviteService.BindReferralForUser(c.Request.Context(), subject.UserID, req.ReferralCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"binding": gin.H{
			"inviter_user_id": binding.InviterUserID,
			"inviter_code":    binding.InviterCode,
			"bound_at":        binding.CreatedAt,
		},
	})
}

// ChangePassword handles changing user password
// POST /api/v1/users/me/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.ChangePasswordRequest{
		CurrentPassword: req.OldPassword,
		NewPassword:     req.NewPassword,
	}
	err := h.userService.ChangePassword(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Password changed successfully"})
}

// UpdateProfile handles updating user profile
// PUT /api/v1/users/me
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.UpdateProfileRequest{
		Username: req.Username,
	}
	updatedUser, err := h.userService.UpdateProfile(c.Request.Context(), subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.UserFromService(updatedUser))
}
