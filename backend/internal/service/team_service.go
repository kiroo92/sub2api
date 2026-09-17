package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"net/mail"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrTeamRequestsPending   = infraerrors.Conflict("TEAM_REQUESTS_PENDING", "team requests are still settling; retry dissolution after completion")
	ErrTeamForbidden         = infraerrors.Forbidden("TEAM_FORBIDDEN", "team access denied")
	ErrTeamConflict          = infraerrors.Conflict("TEAM_CONFLICT", "user already belongs to a team or invitation changed")
	ErrTeamUnavailable       = infraerrors.Forbidden("TEAM_UNAVAILABLE", "team or membership is inactive")
	ErrTeamLimit             = infraerrors.TooManyRequests("TEAM_MEMBER_LIMIT", "team member usage limit reached")
	ErrTeamBillingUnrecorded = infraerrors.ServiceUnavailable("TEAM_BILLING_UNRECORDED", "team bill could not be saved for retry")
)

type TeamLimits struct {
	Daily   float64 `json:"daily"`
	Weekly  float64 `json:"weekly"`
	Monthly float64 `json:"monthly"`
}

func (l TeamLimits) Validate() error {
	for _, n := range []float64{l.Daily, l.Weekly, l.Monthly} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e10 {
			return infraerrors.BadRequest("TEAM_LIMIT_INVALID", "limits must be nonnegative finite amounts")
		}
	}
	return nil
}

type TeamUsage struct {
	TeamLimits
	Total          float64    `json:"total"`
	DailyResetAt   *time.Time `json:"daily_reset_at"`
	WeeklyResetAt  *time.Time `json:"weekly_reset_at"`
	MonthlyResetAt *time.Time `json:"monthly_reset_at"`
}
type Team struct {
	ID            int64      `json:"id"`
	OwnerID       int64      `json:"owner_id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	GroupIDs      []int64    `json:"group_ids"`
	DefaultLimits TeamLimits `json:"default_limits"`
	CreatedAt     time.Time  `json:"created_at"`
}
type TeamMember struct {
	ID       int64      `json:"id"`
	UserID   int64      `json:"user_id"`
	Email    string     `json:"email"`
	Username string     `json:"username"`
	Role     string     `json:"role"`
	Active   bool       `json:"active"`
	Limits   TeamLimits `json:"limits"`
	Usage    TeamUsage  `json:"usage"`
	Resets   TeamResets `json:"resets"`
	JoinedAt time.Time  `json:"joined_at"`
}
type TeamResets struct {
	Daily   *time.Time `json:"daily"`
	Weekly  *time.Time `json:"weekly"`
	Monthly *time.Time `json:"monthly"`
}
type TeamInvitation struct {
	ID        int64     `json:"id"`
	TeamID    int64     `json:"team_id"`
	TeamName  string    `json:"team_name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
type TeamKey struct {
	ID         int64      `json:"id"`
	MemberID   int64      `json:"member_id"`
	UserID     int64      `json:"user_id"`
	Name       string     `json:"name"`
	Key        string     `json:"key"`
	Status     string     `json:"status"`
	Usage      float64    `json:"quota_used"`
	Email      string     `json:"email"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
type TeamGroup struct {
	ID               int64   `json:"id"`
	Name             string  `json:"name"`
	Platform         string  `json:"platform"`
	SubscriptionType string  `json:"subscription_type"`
	RateMultiplier   float64 `json:"rate_multiplier"`
}
type TeamSnapshot struct {
	PendingBilling     int              `json:"pending_billing"`
	PendingRequests    int              `json:"pending_requests"`
	Usage              TeamUsage        `json:"usage"`
	Team               *Team            `json:"team"`
	MyMemberID         int64            `json:"my_member_id"`
	Role               string           `json:"role"`
	Members            []TeamMember     `json:"members"`
	Invitations        []TeamInvitation `json:"invitations"`
	PendingInvitations []TeamInvitation `json:"pending_invitations"`
	AvailableGroups    []TeamGroup      `json:"available_groups"`
}
type TeamSettings struct {
	Name          *string     `json:"name"`
	Status        *string     `json:"status"`
	GroupIDs      *[]int64    `json:"group_ids"`
	DefaultLimits *TeamLimits `json:"default_limits"`
}

// TeamAttribution is request-local. Never put the payer or this snapshot in auth caches.
type TeamAttribution struct {
	TeamID       int64
	MemberID     int64
	MemberUserID int64
	OwnerID      int64
	GroupIDs     []int64
	RequestID    string
}
type TeamRepository interface {
	AdminList(context.Context, string, string, int, int) ([]TeamAdminItem, int, error)
	AdminOwner(context.Context, int64) (int64, error)
	Snapshot(context.Context, int64) (*TeamSnapshot, error)
	Create(context.Context, int64, string) error
	Update(context.Context, int64, TeamSettings) error
	Invite(context.Context, int64, string, string, int64) (*TeamInvitation, error)
	RevokeInvite(context.Context, int64, int64) error
	Accept(context.Context, int64, string, int64) error
	SetLimits(context.Context, int64, int64, TeamLimits) error
	Leave(context.Context, int64, int64) error
	Dissolve(context.Context, int64) error
	Keys(context.Context, int64) ([]TeamKey, error)
	CreateKey(context.Context, int64, string, string) (*TeamKey, error)
	UpdateKey(context.Context, int64, int64, string, string, bool) error
	KeyAttribution(context.Context, int64) (*TeamAttribution, error)
	Admit(context.Context, *TeamAttribution, int64) error
	Release(context.Context, string) error
	CloseRequest(context.Context, string, bool) error
	RecoverBilling(context.Context, int64) (int, error)
}
type TeamService struct {
	repo          TeamRepository
	keys          *APIKeyService
	settings      SettingRepository
	notifications *NotificationEmailService
	billing       *BillingCacheService
}

func NewTeamService(repo TeamRepository, keys *APIKeyService, settings SettingRepository, notifications *NotificationEmailService, billing *BillingCacheService) *TeamService {
	return &TeamService{repo: repo, keys: keys, settings: settings, notifications: notifications, billing: billing}
}

func (s *TeamService) RecoverBilling(ctx context.Context, id int64) (int, error) {
	count, err := s.repo.RecoverBilling(ctx, id)
	if err != nil {
		return count, err
	}
	// Invalidate even on an idempotent retry after a committed response was lost.
	if s.keys != nil {
		s.keys.InvalidateAuthCacheByUserID(ctx, id)
	}
	if s.billing != nil {
		if err = s.billing.InvalidateUserBalance(ctx, id); err != nil {
			return count, err
		}
	}
	return count, nil
}
func (s *TeamService) Snapshot(ctx context.Context, id int64) (*TeamSnapshot, error) {
	out, err := s.repo.Snapshot(ctx, id)
	if err != nil {
		return nil, err
	}
	if out.Team != nil && out.Role == "owner" {
		groups, err := s.keys.GetAvailableGroups(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			out.AvailableGroups = append(out.AvailableGroups, TeamGroup{g.ID, g.Name, g.Platform, g.SubscriptionType, g.RateMultiplier})
		}
	}
	return out, nil
}
func teamName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return "", infraerrors.BadRequest("TEAM_NAME_INVALID", "team name must contain 1–100 characters")
	}
	return name, nil
}
func (s *TeamService) Create(ctx context.Context, id int64, name string) error {
	name, err := teamName(name)
	if err != nil {
		return err
	}
	return s.repo.Create(ctx, id, name)
}
func (s *TeamService) Update(ctx context.Context, id int64, in TeamSettings) error {
	if in.Name != nil {
		n, err := teamName(*in.Name)
		if err != nil {
			return err
		}
		in.Name = &n
	}
	if in.Status != nil && *in.Status != "active" && *in.Status != "paused" {
		return infraerrors.BadRequest("TEAM_STATUS_INVALID", "invalid team status")
	}
	if in.DefaultLimits != nil {
		if err := in.DefaultLimits.Validate(); err != nil {
			return err
		}
	}
	if in.GroupIDs != nil {
		groups, err := s.keys.GetAvailableGroups(ctx, id)
		if err != nil {
			return err
		}
		allowed := map[int64]bool{}
		for _, g := range groups {
			allowed[g.ID] = true
		}
		seen := map[int64]bool{}
		for _, gid := range *in.GroupIDs {
			if !allowed[gid] || seen[gid] {
				return ErrGroupNotAllowed
			}
			seen[gid] = true
		}
	}
	return s.repo.Update(ctx, id, in)
}
func (s *TeamService) Invite(ctx context.Context, id int64, email string, inviteID int64) error {
	base, err := s.InvitationBaseURL(ctx)
	if err != nil {
		return err
	}
	if _, err = teamInvitationURL(base, ""); err != nil {
		return err
	}
	if s.notifications == nil {
		return ErrServiceUnavailable
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if inviteID == 0 {
		a, err := mail.ParseAddress(email)
		if err != nil || a.Address != email || len(email) > 320 {
			return infraerrors.BadRequest("TEAM_EMAIL_INVALID", "invalid email")
		}
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(token))
	inv, err := s.repo.Invite(ctx, id, email, hex.EncodeToString(hash[:]), inviteID)
	if err != nil {
		return err
	}
	// Delivery errors are reported; the owner can resend the persisted invitation.
	link, err := teamInvitationURL(base, token)
	if err != nil {
		return err
	}
	return s.notifications.Send(ctx, NotificationEmailSendInput{Event: NotificationEmailEventTeamInvitation, RecipientEmail: inv.Email, Variables: map[string]string{"team_name": inv.TeamName, "invitation_url": link, "expiry_time": inv.ExpiresAt.Format(time.RFC3339)}})
}

func teamInvitationURL(base, token string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u == nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", infraerrors.BadRequest("TEAM_INVITATION_DOMAIN_REQUIRED", "Configure a valid frontend URL in administrator settings before sending invitations")
	}
	u = u.JoinPath("team")
	u.Fragment = "invite=" + url.QueryEscape(token)
	return u.String(), nil
}
func (s *TeamService) InvitationBaseURL(ctx context.Context) (string, error) {
	value, err := s.settings.GetValue(ctx, SettingKeyFrontendURL)
	if errors.Is(err, ErrSettingNotFound) {
		return "", nil
	}
	return value, err
}
func (s *TeamService) SetInvitationBaseURL(ctx context.Context, value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		if _, err := teamInvitationURL(value, ""); err != nil {
			return err
		}
	}
	return s.settings.Set(ctx, SettingKeyFrontendURL, value)
}
func (s *TeamService) Accept(ctx context.Context, id int64, token string, inviteID int64) error {
	if token != "" {
		decoded, err := hex.DecodeString(token)
		if err != nil || len(decoded) != 32 {
			return ErrTeamForbidden
		}
	}
	hash := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return s.repo.Accept(ctx, id, hex.EncodeToString(hash[:]), inviteID)
}
func (s *TeamService) SetLimits(ctx context.Context, id, member int64, l TeamLimits) error {
	if err := l.Validate(); err != nil {
		return err
	}
	return s.repo.SetLimits(ctx, id, member, l)
}
func (s *TeamService) Keys(ctx context.Context, id int64) ([]TeamKey, error) {
	return s.repo.Keys(ctx, id)
}
func (s *TeamService) CreateKey(ctx context.Context, id int64, name string) (*TeamKey, error) {
	name, err := teamName(name)
	if err != nil {
		return nil, err
	}
	key, err := s.keys.GenerateKey()
	if err != nil {
		return nil, err
	}
	return s.repo.CreateKey(ctx, id, name, key)
}
func (s *TeamService) UpdateKey(ctx context.Context, id, keyID int64, name, status string, remove bool) error {
	if !remove {
		var err error
		name, err = teamName(name)
		if err != nil {
			return err
		}
		if status != "active" && status != "disabled" {
			return infraerrors.BadRequest("TEAM_KEY_STATUS_INVALID", "invalid key status")
		}
	}
	keys, err := s.repo.Keys(ctx, id)
	if err != nil {
		return err
	}
	if err = s.repo.UpdateKey(ctx, id, keyID, name, status, remove); err != nil {
		return err
	}
	for _, key := range keys {
		if key.ID == keyID {
			s.keys.InvalidateAuthCacheByKey(context.WithoutCancel(ctx), key.Key)
			break
		}
	}
	return nil
}
func (s *TeamService) RevokeInvite(ctx context.Context, id, invite int64) error {
	return s.repo.RevokeInvite(ctx, id, invite)
}
func (s *TeamService) Leave(ctx context.Context, id, member int64) error {
	return s.repo.Leave(ctx, id, member)
}
func (s *TeamService) Dissolve(ctx context.Context, id int64, name string) error {
	out, err := s.repo.Snapshot(ctx, id)
	if err != nil {
		return err
	}
	if out.Team == nil || out.Role != "owner" || out.Team.Name != name {
		return infraerrors.BadRequest("TEAM_CONFIRMATION_REQUIRED", "enter the team name to confirm dissolution")
	}
	return s.repo.Dissolve(ctx, id)
}
