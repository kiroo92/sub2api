package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"net/url"
	"regexp"
	"testing"
	"time"
)

type teamInviteRepo struct {
	TeamRepository
	calls int
	hash  string
}

func (r *teamInviteRepo) Invite(_ context.Context, _ int64, email, hash string, _ int64) (*TeamInvitation, error) {
	r.calls++
	r.hash = hash
	return &TeamInvitation{Email: email, TeamName: "<Team>", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func TestTeamInvitationDomainTemplateAndSend(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	repo := &teamInviteRepo{}
	smtp := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, settings.SetMultiple(ctx, smtp.settings()))
	notifications := NewNotificationEmailService(settings, NewEmailService(settings, nil))
	svc := NewTeamService(repo, nil, settings, notifications, nil)
	require.Error(t, svc.Invite(ctx, 1, "member@example.com", 0))
	require.Zero(t, repo.calls)
	require.Error(t, svc.Invite(ctx, 1, "", 9))
	require.Zero(t, repo.calls, "resend also requires domain before rotating token")
	for _, base := range []string{"", "javascript:alert(1)", "//example.com", "https://user:pass@example.com", "https://example.com?next=evil", "https://example.com#fragment"} {
		_, err := teamInvitationURL(base, "token")
		require.Error(t, err, base)
	}
	require.NoError(t, svc.SetInvitationBaseURL(ctx, "https://example.com/app/"))
	require.NoError(t, svc.Invite(ctx, 1, "member@example.com", 0))
	body := smtp.lastMessageBody(t)
	require.Contains(t, body, "&lt;Team&gt;")
	match := regexp.MustCompile(`https://example.com/app/team#invite=([a-f0-9]{64})`).FindStringSubmatch(body)
	require.Len(t, match, 2)
	token, err := url.QueryUnescape(match[1])
	require.NoError(t, err)
	hash := sha256.Sum256([]byte(token))
	require.Equal(t, hex.EncodeToString(hash[:]), repo.hash)
	first := repo.hash
	require.NoError(t, svc.Invite(ctx, 1, "member@example.com", 0))
	require.NotEqual(t, first, repo.hash)
	require.Equal(t, int64(2), smtp.messageCount())
	for _, locale := range []string{"zh", "en"} {
		preview, err := notifications.PreviewTemplate(ctx, NotificationEmailPreviewInput{Event: NotificationEmailEventTeamInvitation, Locale: locale})
		require.NoError(t, err)
		require.Contains(t, preview.HTML, "team#invite=preview")
	}
	_, err = notifications.UpdateTemplate(ctx, NotificationEmailEventTeamInvitation, "en", "Invite", "<p>missing link</p>")
	require.Error(t, err)
	require.NoError(t, svc.SetInvitationBaseURL(ctx, ""))
	require.Error(t, svc.Invite(ctx, 1, "member@example.com", 0))
	require.Equal(t, 2, repo.calls)
}
