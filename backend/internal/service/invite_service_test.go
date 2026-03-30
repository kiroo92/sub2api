//go:build unit

package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type inviteRepoStub struct {
	byUserID          map[int64]*InviteCodeRecord
	byCode            map[string]*InviteCodeRecord
	bindingsByInvitee map[int64]*InviteBindingRecord
	createBindingErr  error
	createdBindings   []InviteBindingRecord
}

func (s *inviteRepoStub) GetInviteCodeByUserID(ctx context.Context, userID int64) (*InviteCodeRecord, error) {
	if rec, ok := s.byUserID[userID]; ok {
		return rec, nil
	}
	return nil, sql.ErrNoRows
}

func (s *inviteRepoStub) GetInviteCodeByCode(ctx context.Context, code string) (*InviteCodeRecord, error) {
	if rec, ok := s.byCode[code]; ok {
		return rec, nil
	}
	return nil, sql.ErrNoRows
}

func (s *inviteRepoStub) CreateInviteCode(ctx context.Context, userID int64, code string) (*InviteCodeRecord, error) {
	rec := &InviteCodeRecord{ID: int64(len(s.byUserID) + 1), UserID: userID, Code: code, Active: true}
	if s.byUserID == nil {
		s.byUserID = map[int64]*InviteCodeRecord{}
	}
	if s.byCode == nil {
		s.byCode = map[string]*InviteCodeRecord{}
	}
		s.byUserID[userID] = rec
		s.byCode[code] = rec
	return rec, nil
}

func (s *inviteRepoStub) GetBindingByInviteeUserID(ctx context.Context, inviteeUserID int64) (*InviteBindingRecord, error) {
	if rec, ok := s.bindingsByInvitee[inviteeUserID]; ok {
		return rec, nil
	}
	return nil, sql.ErrNoRows
}

func (s *inviteRepoStub) CreateBinding(ctx context.Context, inviterUserID, inviteeUserID, inviteCodeID int64) (*InviteBindingRecord, error) {
	if s.createBindingErr != nil {
		return nil, s.createBindingErr
	}
	rec := &InviteBindingRecord{ID: int64(len(s.createdBindings) + 1), InviterUserID: inviterUserID, InviteeUserID: inviteeUserID, InviteCodeID: inviteCodeID, CreatedAt: "2026-03-30T09:00:00Z"}
	if s.bindingsByInvitee == nil {
		s.bindingsByInvitee = map[int64]*InviteBindingRecord{}
	}
	s.bindingsByInvitee[inviteeUserID] = rec
	s.createdBindings = append(s.createdBindings, *rec)
	return rec, nil
}

func TestInviteService_BindReferralForUser_Success(t *testing.T) {
	repo := &inviteRepoStub{
		byCode: map[string]*InviteCodeRecord{
			"ABCD1234": {ID: 10, UserID: 99, Code: "ABCD1234", Active: true},
		},
		bindingsByInvitee: map[int64]*InviteBindingRecord{},
	}
	svc := NewInviteService(repo, nil)

	binding, err := svc.BindReferralForUser(context.Background(), 1001, "ABCD1234")
	require.NoError(t, err)
	require.NotNil(t, binding)
	require.Equal(t, int64(99), binding.InviterUserID)
	require.Equal(t, 1, len(repo.createdBindings))
}

func TestInviteService_BindReferralForUser_SelfBind(t *testing.T) {
	repo := &inviteRepoStub{
		byCode: map[string]*InviteCodeRecord{
			"SELF1234": {ID: 10, UserID: 42, Code: "SELF1234", Active: true},
		},
		bindingsByInvitee: map[int64]*InviteBindingRecord{},
	}
	svc := NewInviteService(repo, nil)

	_, err := svc.BindReferralForUser(context.Background(), 42, "SELF1234")
	require.ErrorIs(t, err, ErrReferralSelfBindForbidden)
}

func TestInviteService_GetInviteInfoForUser_BuildsLink(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.FrontendURL = "https://app.example.com"
	settingSvc := NewSettingService(&settingRepoStub{values: map[string]string{}}, cfg)
	repo := &inviteRepoStub{
		byUserID: map[int64]*InviteCodeRecord{
			7: {ID: 1, UserID: 7, Code: "ZXCV1234", Active: true},
		},
		bindingsByInvitee: map[int64]*InviteBindingRecord{},
	}
	svc := NewInviteService(repo, settingSvc)

	info, err := svc.GetInviteInfoForUser(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "ZXCV1234", info.InviteCode)
	require.Equal(t, "https://app.example.com/register?ref=ZXCV1234", info.InviteLink)
	require.True(t, info.CanBind)
}
