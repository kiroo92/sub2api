package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type teamActivationRepoStub struct {
	TeamRepository
	calls int
	hash  string
	name  string
}

func (r *teamActivationRepoStub) InviteActivation(_ context.Context, _ int64, email, hash string) (*TeamActivationInvitation, error) {
	r.calls++
	r.hash = hash
	return &TeamActivationInvitation{Email: email, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (r *teamActivationRepoStub) Create(_ context.Context, _ int64, name, hash string) error {
	r.calls++
	r.hash = hash
	r.name = name
	return nil
}

func TestTeamActivationRequiresTokenBeforeCreating(t *testing.T) {
	repo := &teamActivationRepoStub{}
	svc := NewTeamService(repo, nil, nil, nil, nil)
	ctx := context.Background()
	for _, token := range []string{"", "invalid", strings.Repeat("a", 63), strings.Repeat("g", 64)} {
		require.ErrorIs(t, svc.Create(ctx, 1, "Enterprise", token), ErrTeamActivationRequired)
	}
	require.Zero(t, repo.calls)
	token := strings.Repeat("a", 64)
	require.NoError(t, svc.Create(ctx, 1, " Enterprise ", token))
	hash := sha256.Sum256([]byte(token))
	require.Equal(t, hex.EncodeToString(hash[:]), repo.hash)
	require.Equal(t, "Enterprise", repo.name)
}

func TestTeamActivationEmailAndTemplate(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	repo := &teamActivationRepoStub{}
	smtp := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, settings.SetMultiple(ctx, smtp.settings()))
	notifications := NewNotificationEmailService(settings, NewEmailService(settings, nil))
	svc := NewTeamService(repo, nil, settings, notifications, nil)
	_, err := svc.InviteActivation(ctx, 1, "owner@example.com")
	require.Error(t, err)
	require.Zero(t, repo.calls)
	require.NoError(t, svc.SetInvitationBaseURL(ctx, "https://example.com/app/"))
	_, err = svc.InviteActivation(ctx, 1, "invalid email")
	require.Error(t, err)
	require.Zero(t, repo.calls)
	link, err := svc.InviteActivation(ctx, 1, " Owner@Example.com ")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(link, "https://example.com/app/team#activate="))
	token := strings.Split(link, "#activate=")[1]
	require.Len(t, token, 64)
	hash := sha256.Sum256([]byte(token))
	require.Equal(t, hex.EncodeToString(hash[:]), repo.hash)
	body := smtp.lastMessageBody(t)
	require.Contains(t, body, link)
	require.Contains(t, body, "owner@example.com")
	second, err := svc.InviteActivation(ctx, 1, "owner@example.com")
	require.NoError(t, err)
	require.NotEqual(t, link, second)
	for _, locale := range []string{"zh", "en"} {
		preview, err := notifications.PreviewTemplate(ctx, NotificationEmailPreviewInput{Event: NotificationEmailEventTeamActivation, Locale: locale})
		require.NoError(t, err)
		require.Contains(t, preview.HTML, "team#activate=preview")
	}
	_, err = notifications.UpdateTemplate(ctx, NotificationEmailEventTeamActivation, "en", "Activation", "<p>missing link</p>")
	require.Error(t, err)
}
