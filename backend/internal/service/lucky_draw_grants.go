package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"strings"
	"unicode/utf8"
)

func (s *LuckyDrawService) GrantChances(ctx context.Context, adminID, userID int64, requestID string, count int, note string) (*LuckyDrawGrant, error) {
	id, err := uuid.Parse(requestID)
	note = strings.TrimSpace(note)
	if err != nil || id == uuid.Nil || adminID <= 0 || userID <= 0 || count < 1 || count > 10000 || utf8.RuneCountInString(note) > 500 {
		return nil, infraerrors.BadRequest("INVALID_LUCKY_DRAW_REQUEST", "请检查用户、赠送次数和备注")
	}
	return s.repo.GrantChances(ctx, adminID, userID, id.String(), count, note)
}
