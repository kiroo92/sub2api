package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invitebinding"
	"github.com/Wei-Shaw/sub2api/ent/invitecode"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type inviteRepository struct {
	client *dbent.Client
	sql    *sql.DB
}

func NewInviteRepository(client *dbent.Client, sqlDB *sql.DB) service.InviteRepository {
	return &inviteRepository{client: client, sql: sqlDB}
}

func (r *inviteRepository) GetInviteCodeByUserID(ctx context.Context, userID int64) (*service.InviteCodeRecord, error) {
	client := clientFromContext(ctx, r.client)
	record, err := client.InviteCode.Query().Where(invitecode.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &service.InviteCodeRecord{ID: record.ID, UserID: record.UserID, Code: record.Code, Active: record.Active}, nil
}

func (r *inviteRepository) GetInviteCodeByCode(ctx context.Context, code string) (*service.InviteCodeRecord, error) {
	client := clientFromContext(ctx, r.client)
	record, err := client.InviteCode.Query().Where(invitecode.CodeEQ(code)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &service.InviteCodeRecord{ID: record.ID, UserID: record.UserID, Code: record.Code, Active: record.Active}, nil
}

func (r *inviteRepository) CreateInviteCode(ctx context.Context, userID int64, code string) (*service.InviteCodeRecord, error) {
	client := clientFromContext(ctx, r.client)
	record, err := client.InviteCode.Create().SetUserID(userID).SetCode(code).SetActive(true).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &service.InviteCodeRecord{ID: record.ID, UserID: record.UserID, Code: record.Code, Active: record.Active}, nil
}

func (r *inviteRepository) GetBindingByInviteeUserID(ctx context.Context, inviteeUserID int64) (*service.InviteBindingRecord, error) {
	client := clientFromContext(ctx, r.client)
	record, err := client.InviteBinding.Query().Where(invitebinding.InviteeUserIDEQ(inviteeUserID)).WithInviteCode().Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	inviteCode := ""
	if edges := record.Edges.InviteCode; edges != nil {
		inviteCode = edges.Code
	}
	return &service.InviteBindingRecord{
		ID:            record.ID,
		InviterUserID: record.InviterUserID,
		InviteeUserID: record.InviteeUserID,
		InviteCodeID:  record.InviteCodeID,
		InviterCode:   inviteCode,
		CreatedAt:     record.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (r *inviteRepository) CreateBinding(ctx context.Context, inviterUserID, inviteeUserID, inviteCodeID int64) (*service.InviteBindingRecord, error) {
	client := clientFromContext(ctx, r.client)
	record, err := client.InviteBinding.Create().
		SetInviterUserID(inviterUserID).
		SetInviteeUserID(inviteeUserID).
		SetInviteCodeID(inviteCodeID).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &service.InviteBindingRecord{
		ID:            record.ID,
		InviterUserID: record.InviterUserID,
		InviteeUserID: record.InviteeUserID,
		InviteCodeID:  record.InviteCodeID,
		CreatedAt:     record.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

var _ service.InviteRepository = (*inviteRepository)(nil)

func (r *inviteRepository) String() string {
	return fmt.Sprintf("inviteRepository(%p)", r)
}
