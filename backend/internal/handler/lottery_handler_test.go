package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type lotteryHandlerRepo struct{ userID, roundID int64 }

func (r *lotteryHandlerRepo) Snapshot(context.Context, int64) (*service.LotterySnapshot, error) {
	return &service.LotterySnapshot{}, nil
}
func (r *lotteryHandlerRepo) Configure(context.Context, service.LotteryConfig) error { return nil }
func (r *lotteryHandlerRepo) Join(_ context.Context, userID, roundID int64, _ service.LotteryWinnerPicker) (*service.LotteryJoinResult, error) {
	r.userID = userID
	r.roundID = roundID
	return &service.LotteryJoinResult{RoundID: roundID}, nil
}
func TestLotteryHandlerUsesAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &lotteryHandlerRepo{}
	h := NewLotteryHandler(service.NewLotteryService(repo, nil, nil), &config.Config{}, nil)
	h.captcha = &lotteryCaptchaStub{}
	c, w := lotteryHandlerContext(`{"round_id":131,"user_id":999,"tencent_captcha_ticket":"ticket","tencent_captcha_randstr":"rand"}`)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.Join(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(42), repo.userID)
	require.Equal(t, int64(131), repo.roundID)
	require.Equal(t, service.CaptchaProof{TencentTicket: "ticket", TencentRandstr: "rand"}, h.captcha.(*lotteryCaptchaStub).proof)
}
func TestLotteryHandlerRejectsInvalidEntry(t *testing.T) {
	for _, body := range []string{`{}`, `{"round_id":0}`, `{"round_id":-1}`, `{"round_id":"131"}`} {
		repo := &lotteryHandlerRepo{}
		h := NewLotteryHandler(service.NewLotteryService(repo, nil, nil), &config.Config{}, nil)
		h.captcha = &lotteryCaptchaStub{}
		c, w := lotteryHandlerContext(body)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		h.Join(c)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Zero(t, repo.userID)
	}
	c, w := lotteryHandlerContext(`{"round_id":1}`)
	h := NewLotteryHandler(nil, &config.Config{}, nil)
	h.Join(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	c, w = lotteryHandlerContext(`{"round_id":1}`)
	h = NewLotteryHandler(nil, &config.Config{RunMode: config.RunModeSimple}, nil)
	h.Join(c)
	require.Equal(t, http.StatusForbidden, w.Code)
}
func lotteryHandlerContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/lottery/join", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

type lotteryCaptchaStub struct {
	err   error
	proof service.CaptchaProof
}

func (s *lotteryCaptchaStub) VerifyLotteryCaptcha(_ context.Context, p service.CaptchaProof, _ string) error {
	s.proof = p
	return s.err
}
func TestLotteryHandlerCaptchaFailureBlocksEntry(t *testing.T) {
	repo := &lotteryHandlerRepo{}
	h := NewLotteryHandler(service.NewLotteryService(repo, nil, nil), &config.Config{}, nil)
	h.captcha = &lotteryCaptchaStub{err: service.ErrTencentCaptchaVerificationFailed}
	c, w := lotteryHandlerContext(`{"round_id":131}`)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	h.Join(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Zero(t, repo.userID)
}
