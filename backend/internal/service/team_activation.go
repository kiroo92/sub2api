package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/mail"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrTeamActivationRequired = infraerrors.Forbidden("TEAM_ACTIVATION_REQUIRED", "Enterprise activation requires a valid invitation link from a platform administrator for your email address")

type TeamActivationInvitation struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *TeamService) ActivationInvitations(ctx context.Context) ([]TeamActivationInvitation, error) {
	return s.repo.ActivationInvitations(ctx)
}

func (s *TeamService) InviteActivation(ctx context.Context, adminID int64, email string) (string, error) {
	base, err := s.InvitationBaseURL(ctx)
	if err != nil {
		return "", err
	}
	if _, err := teamInvitationURL(base, ""); err != nil {
		return "", err
	}
	if s.notifications == nil {
		return "", ErrServiceUnavailable
	}
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 320 {
		return "", infraerrors.BadRequest("TEAM_EMAIL_INVALID", "invalid email")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(token))
	inv, err := s.repo.InviteActivation(ctx, adminID, email, hex.EncodeToString(hash[:]))
	if err != nil {
		return "", err
	}
	link, err := teamInvitationURL(base, token)
	if err != nil {
		return "", err
	}
	link = strings.Replace(link, "#invite=", "#activate=", 1)
	err = s.notifications.Send(ctx, NotificationEmailSendInput{Event: NotificationEmailEventTeamActivation, RecipientEmail: inv.Email, Variables: map[string]string{"invitation_url": link, "expiry_time": inv.ExpiresAt.Format(time.RFC3339)}})
	if err != nil {
		return "", err
	}
	return link, nil
}

func (s *TeamService) RevokeActivation(ctx context.Context, id int64) error {
	return s.repo.RevokeActivation(ctx, id)
}
