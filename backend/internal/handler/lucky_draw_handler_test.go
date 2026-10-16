package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type luckyHandlerRepo struct{ userID int64 }

func (r *luckyHandlerRepo) Snapshot(_ context.Context, userID int64) (*service.LuckyDrawSnapshot, error) {
	r.userID = userID
	return &service.LuckyDrawSnapshot{}, nil
}
func (r *luckyHandlerRepo) Configure(context.Context, service.LuckyDrawConfig) error { return nil }
func (r *luckyHandlerRepo) Draw(_ context.Context, userID int64, requestID string, version int64, _ service.LuckyDrawPicker) (*service.LuckyDrawRecord, error) {
	r.userID = userID
	return &service.LuckyDrawRecord{RequestID: requestID, ConfigVersion: version}, nil
}
func TestLuckyDrawHandlerUsesSessionIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &luckyHandlerRepo{}
	h := NewLuckyDrawHandler(service.NewLuckyDrawService(repo, nil, nil), &config.Config{})
	c, w := lotteryHandlerContext(`{"user_id":999,"request_id":"d66d1ae0-9bea-4567-8ab7-9198e788cb6f","config_version":2}`)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.Draw(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.EqualValues(t, 42, repo.userID)
	c, w = lotteryHandlerContext("")
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.Get(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.EqualValues(t, 42, repo.userID)
}
func TestLuckyDrawHandlerRejectsInvalidRequest(t *testing.T) {
	for _, body := range []string{`{}`, `{"request_id":"invalid","config_version":2}`, `{"request_id":"d66d1ae0-9bea-4567-8ab7-9198e788cb6f","config_version":0}`, `{"request_id":"00000000-0000-0000-0000-000000000000","config_version":2}`} {
		repo := &luckyHandlerRepo{}
		h := NewLuckyDrawHandler(service.NewLuckyDrawService(repo, nil, nil), &config.Config{})
		c, w := lotteryHandlerContext(body)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		h.Draw(c)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Zero(t, repo.userID)
	}
	c, w := lotteryHandlerContext(`{}`)
	h := NewLuckyDrawHandler(nil, &config.Config{})
	h.Draw(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	c, w = lotteryHandlerContext(`{}`)
	h = NewLuckyDrawHandler(nil, &config.Config{RunMode: config.RunModeSimple})
	h.Draw(c)
	require.Equal(t, http.StatusForbidden, w.Code)
}
