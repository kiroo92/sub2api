package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TeamHandler struct{ service *service.TeamService }

func (h *TeamHandler) RecoverBilling(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	count, err := h.service.RecoverBilling(c.Request.Context(), id)
	teamResult(c, gin.H{"recovered": count}, err)
}

func NewTeamHandler(s *service.TeamService) *TeamHandler { return &TeamHandler{s} }
func teamSubject(c *gin.Context) (int64, bool) {
	if owner, ok := c.Get(teamAdminOwnerContext); ok {
		if !teamPlatformAdmin(c) {
			return 0, false
		}
		id, valid := owner.(int64)
		return id, valid && id > 0
	}
	s, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
	}
	return s.UserID, ok
}
func teamParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid ID")
		return 0, false
	}
	return id, true
}
func teamResult(c *gin.Context, data any, err error) {
	c.Header("Cache-Control", "no-store")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if data == nil {
		data = gin.H{"success": true}
	}
	response.Success(c, data)
}
func (h *TeamHandler) Get(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	out, err := h.service.Snapshot(c.Request.Context(), id)
	teamResult(c, out, err)
}
func (h *TeamHandler) Groups(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	out, err := h.service.Snapshot(c.Request.Context(), id)
	if err != nil {
		teamResult(c, nil, err)
		return
	}
	if out.Role != "owner" {
		teamResult(c, nil, service.ErrTeamForbidden)
		return
	}
	teamResult(c, out.AvailableGroups, nil)
}
func (h *TeamHandler) Create(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name" binding:"required"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Team name is required")
		return
	}
	teamResult(c, nil, h.service.Create(c.Request.Context(), id, in.Name))
}
func (h *TeamHandler) Update(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	var in service.TeamSettings
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Invalid team settings")
		return
	}
	teamResult(c, nil, h.service.Update(c.Request.Context(), id, in))
}
func (h *TeamHandler) Dissolve(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"confirm_name" binding:"required"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Confirm team name")
		return
	}
	teamResult(c, nil, h.service.Dissolve(c.Request.Context(), id, in.Name))
}
func (h *TeamHandler) Leave(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	teamResult(c, nil, h.service.Leave(c.Request.Context(), id, 0))
}
func (h *TeamHandler) RemoveMember(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	member, ok := teamParam(c, "user_id")
	if !ok {
		return
	}
	teamResult(c, nil, h.service.Leave(c.Request.Context(), id, member))
}
func (h *TeamHandler) Limits(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	member, ok := teamParam(c, "user_id")
	if !ok {
		return
	}
	var in service.TeamLimits
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Invalid limits")
		return
	}
	teamResult(c, nil, h.service.SetLimits(c.Request.Context(), id, member, in))
}
func (h *TeamHandler) Invite(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	var in struct {
		Email string `json:"email" binding:"required"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Email is required")
		return
	}
	teamResult(c, nil, h.service.Invite(c.Request.Context(), id, in.Email, 0))
}
func (h *TeamHandler) Resend(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	invite, ok := teamParam(c, "id")
	if !ok {
		return
	}
	teamResult(c, nil, h.service.Invite(c.Request.Context(), id, "", invite))
}
func (h *TeamHandler) RevokeInvite(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	invite, ok := teamParam(c, "id")
	if !ok {
		return
	}
	teamResult(c, nil, h.service.RevokeInvite(c.Request.Context(), id, invite))
}
func (h *TeamHandler) Accept(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	var in struct {
		Token        string `json:"token"`
		InvitationID int64  `json:"invitation_id"`
	}
	if c.ShouldBindJSON(&in) != nil || (in.Token == "" && in.InvitationID <= 0) {
		response.BadRequest(c, "Invitation is required")
		return
	}
	teamResult(c, nil, h.service.Accept(c.Request.Context(), id, in.Token, in.InvitationID))
}
func (h *TeamHandler) Keys(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	out, err := h.service.Keys(c.Request.Context(), id)
	teamResult(c, out, err)
}
func (h *TeamHandler) CreateKey(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name" binding:"required"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Key name is required")
		return
	}
	out, err := h.service.CreateKey(c.Request.Context(), id, in.Name)
	teamResult(c, out, err)
}
func (h *TeamHandler) UpdateKey(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	key, ok := teamParam(c, "id")
	if !ok {
		return
	}
	var in struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "Invalid key")
		return
	}
	keys, err := h.service.Keys(c.Request.Context(), id)
	if err != nil {
		teamResult(c, nil, err)
		return
	}
	for _, k := range keys {
		if k.ID == key {
			if in.Name != nil {
				k.Name = *in.Name
			}
			if in.Status != nil {
				k.Status = *in.Status
			}
			teamResult(c, nil, h.service.UpdateKey(c.Request.Context(), id, key, k.Name, k.Status, false))
			return
		}
	}
	teamResult(c, nil, service.ErrTeamForbidden)
}
func (h *TeamHandler) DeleteKey(c *gin.Context) {
	id, ok := teamSubject(c)
	if !ok {
		return
	}
	key, ok := teamParam(c, "id")
	if !ok {
		return
	}
	teamResult(c, nil, h.service.UpdateKey(c.Request.Context(), id, key, "", "", true))
}
