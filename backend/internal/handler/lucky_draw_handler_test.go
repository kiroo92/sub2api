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

type luckyHandlerRepo struct {
	userID, adminID int64
	grantCount      int
}

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

func (r *luckyHandlerRepo) AdminStats(context.Context, string, int) (*service.LuckyDrawAdminStats, error) {
	return &service.LuckyDrawAdminStats{}, nil
}

func TestLuckyDrawStatsRejectsInvalidPageAndSimpleMode(t *testing.T) {
	h := NewLuckyDrawHandler(service.NewLuckyDrawService(&luckyHandlerRepo{}, nil, nil), &config.Config{})
	for _, query := range []string{"page=0", "page=-1", "page=abc", "page=1000001"} {
		c, w := lotteryHandlerContext("")
		c.Request.URL.RawQuery = query
		h.Stats(c)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
	c, w := lotteryHandlerContext("")
	h.Stats(c)
	require.Equal(t, http.StatusOK, w.Code)
	c, w = lotteryHandlerContext("")
	h = NewLuckyDrawHandler(nil, &config.Config{RunMode: config.RunModeSimple})
	h.Stats(c)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func (r *luckyHandlerRepo) GrantChances(_ context.Context, adminID, userID int64, _ string, count int, _ string) (*service.LuckyDrawGrant, error) {
	r.adminID = adminID
	r.userID = userID
	r.grantCount = count
	return &service.LuckyDrawGrant{AdminID: adminID, UserID: userID, Chances: count}, nil
}
func TestLuckyDrawGrantUsesAuthenticatedAdmin(t *testing.T) {
	repo := &luckyHandlerRepo{}
	h := NewLuckyDrawHandler(service.NewLuckyDrawService(repo, nil, nil), &config.Config{})
	c, w := lotteryHandlerContext(`{"admin_id":999,"user_id":7,"request_id":"d66d1ae0-9bea-4567-8ab7-9198e788cb6f","count":10,"note":"活动补发"}`)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.Grant(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.EqualValues(t, 42, repo.adminID)
	require.EqualValues(t, 7, repo.userID)
	require.Equal(t, 10, repo.grantCount)
	c, w = lotteryHandlerContext(`{}`)
	h.Grant(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	for _, body := range []string{`{"user_id":7,"request_id":"d66d1ae0-9bea-4567-8ab7-9198e788cb6f","count":0}`, `{"user_id":7,"request_id":"d66d1ae0-9bea-4567-8ab7-9198e788cb6f","count":10001}`} {
		c, w = lotteryHandlerContext(body)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		h.Grant(c)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func (r *luckyHandlerRepo) DrawBatch(ctx context.Context, userID int64, requestID string, version int64, count int, pick service.LuckyDrawPicker) (*service.LuckyDrawBatch, error) {
	d, err := r.Draw(ctx, userID, requestID, version, pick)
	if err != nil {
		return nil, err
	}
	return &service.LuckyDrawBatch{Records: []service.LuckyDrawRecord{*d}}, nil
}
