package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const inviteCodeLength = 8
const maxGenerateInviteCodeAttempts = 12

var (
	ErrReferralCodeInvalid       = infraerrors.BadRequest("REFERRAL_CODE_INVALID", "invalid referral code")
	ErrReferralCodeInactive      = infraerrors.BadRequest("REFERRAL_CODE_INACTIVE", "referral code is inactive")
	ErrReferralAlreadyBound      = infraerrors.Conflict("REFERRAL_ALREADY_BOUND", "user already has a referrer")
	ErrReferralSelfBindForbidden = infraerrors.BadRequest("REFERRAL_SELF_BIND_FORBIDDEN", "cannot bind your own referral code")
	ErrReferralMutualBindBlocked = infraerrors.Conflict("REFERRAL_MUTUAL_BIND_FORBIDDEN", "mutual referral binding is not allowed")
)

type InviteInfo struct {
	InviteCode  string             `json:"invite_code"`
	InviteLink  string             `json:"invite_link,omitempty"`
	Binding     *InviteBindingInfo `json:"binding,omitempty"`
	CanBind     bool               `json:"can_bind"`
}

type InviteBindingInfo struct {
	InviterUserID int64  `json:"inviter_user_id"`
	InviterCode   string `json:"inviter_code"`
	BoundAt       string `json:"bound_at"`
}

type InviteCodeRecord struct {
	ID     int64
	UserID int64
	Code   string
	Active bool
}

type InviteBindingRecord struct {
	ID            int64
	InviterUserID int64
	InviteeUserID int64
	InviteCodeID  int64
	InviterCode   string
	CreatedAt     string
}

type InviteRepository interface {
	GetInviteCodeByUserID(ctx context.Context, userID int64) (*InviteCodeRecord, error)
	GetInviteCodeByCode(ctx context.Context, code string) (*InviteCodeRecord, error)
	CreateInviteCode(ctx context.Context, userID int64, code string) (*InviteCodeRecord, error)
	GetBindingByInviteeUserID(ctx context.Context, inviteeUserID int64) (*InviteBindingRecord, error)
	CreateBinding(ctx context.Context, inviterUserID, inviteeUserID, inviteCodeID int64) (*InviteBindingRecord, error)
}

type InviteService struct {
	repo          InviteRepository
	settingService *SettingService
}

func NewInviteService(repo InviteRepository, settingService *SettingService) *InviteService {
	return &InviteService{repo: repo, settingService: settingService}
}

func normalizeReferralCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func generateInviteCodeCandidate() (string, error) {
	buf := make([]byte, inviteCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := make([]byte, inviteCodeLength)
	for i := range buf {
		code[i] = inviteCodeAlphabet[int(buf[i])%len(inviteCodeAlphabet)]
	}
	return string(code), nil
}

func (s *InviteService) EnsureInviteCodeForUser(ctx context.Context, userID int64) (*InviteCodeRecord, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("invite service not configured")
	}
	if existing, err := s.repo.GetInviteCodeByUserID(ctx, userID); err == nil {
		return existing, nil
	} else if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	for attempt := 0; attempt < maxGenerateInviteCodeAttempts; attempt++ {
		candidate, err := generateInviteCodeCandidate()
		if err != nil {
			return nil, err
		}
		record, err := s.repo.CreateInviteCode(ctx, userID, candidate)
		if err == nil {
			return record, nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			if existing, lookupErr := s.repo.GetInviteCodeByUserID(ctx, userID); lookupErr == nil {
				return existing, nil
			}
			continue
		}
		return nil, err
	}

	return nil, fmt.Errorf("generate invite code: exhausted retries")
}

func (s *InviteService) BindReferralForUser(ctx context.Context, userID int64, referralCode string) (*InviteBindingRecord, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("invite service not configured")
	}
	code := normalizeReferralCode(referralCode)
	if code == "" {
		return nil, ErrReferralCodeInvalid
	}

	existingBinding, err := s.repo.GetBindingByInviteeUserID(ctx, userID)
	if err == nil && existingBinding != nil {
		return nil, ErrReferralAlreadyBound
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	inviteCode, err := s.repo.GetInviteCodeByCode(ctx, code)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrReferralCodeInvalid
		}
		return nil, err
	}
	if !inviteCode.Active {
		return nil, ErrReferralCodeInactive
	}
	if inviteCode.UserID == userID {
		return nil, ErrReferralSelfBindForbidden
	}

	reverseBinding, err := s.repo.GetBindingByInviteeUserID(ctx, inviteCode.UserID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if reverseBinding != nil && reverseBinding.InviterUserID == userID {
		return nil, ErrReferralMutualBindBlocked
	}

	binding, err := s.repo.CreateBinding(ctx, inviteCode.UserID, userID, inviteCode.ID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrReferralAlreadyBound
		}
		return nil, err
	}
	binding.InviterCode = inviteCode.Code
	return binding, nil
}

func (s *InviteService) GetInviteInfoForUser(ctx context.Context, userID int64) (*InviteInfo, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("invite service not configured")
	}
	inviteCode, err := s.EnsureInviteCodeForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var bindingInfo *InviteBindingInfo
	binding, err := s.repo.GetBindingByInviteeUserID(ctx, userID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if binding != nil {
		bindingInfo = &InviteBindingInfo{
			InviterUserID: binding.InviterUserID,
			InviterCode:   binding.InviterCode,
			BoundAt:       binding.CreatedAt,
		}
	}

	inviteLink := ""
	if s.settingService != nil {
		frontendURL := strings.TrimRight(s.settingService.GetFrontendURL(ctx), "/")
		if frontendURL != "" {
			inviteLink = fmt.Sprintf("%s/register?ref=%s", frontendURL, inviteCode.Code)
		}
	}

	return &InviteInfo{
		InviteCode: inviteCode.Code,
		InviteLink: inviteLink,
		Binding:    bindingInfo,
		CanBind:    bindingInfo == nil,
	}, nil
}
