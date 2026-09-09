package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type lotteryCaptchaRepoStub struct{ config LotteryConfig }

func (r *lotteryCaptchaRepoStub) Snapshot(context.Context, int64) (*LotterySnapshot, error) {
	return &LotterySnapshot{Config: r.config}, nil
}
func (r *lotteryCaptchaRepoStub) Configure(context.Context, LotteryConfig) error { return nil }
func (r *lotteryCaptchaRepoStub) Join(context.Context, int64, int64, LotteryWinnerPicker) (*LotteryJoinResult, error) {
	return nil, nil
}

type lotteryTurnstileStub struct {
	secret, token string
	success       bool
	err           error
}

func (v *lotteryTurnstileStub) VerifyToken(_ context.Context, secret, token, ip string) (*TurnstileVerifyResponse, error) {
	v.secret = secret
	v.token = token
	return &TurnstileVerifyResponse{Success: v.success}, v.err
}
func TestLotteryCaptchaIndependent(t *testing.T) {
	repo := &lotteryCaptchaRepoStub{config: LotteryConfig{TurnstileSiteKey: "dedicated-site", TurnstileSecretKey: "dedicated-secret"}}
	verifier := &lotteryTurnstileStub{success: true}
	svc := NewLotteryCaptchaService(repo, verifier)
	require.NoError(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{TurnstileToken: "proof"}, "127.0.0.1"))
	require.Equal(t, "dedicated-secret", verifier.secret)
	require.ErrorIs(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{TencentTicket: "ticket"}, ""), ErrTurnstileVerificationFailed)
	verifier.success = false
	require.ErrorIs(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{TurnstileToken: "expired"}, ""), ErrTurnstileVerificationFailed)
	verifier.err = errors.New("network failure")
	require.ErrorIs(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{TurnstileToken: "proof"}, ""), ErrServiceUnavailable)
	repo.config.TurnstileSecretKey = ""
	require.Error(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{TurnstileToken: "proof"}, ""))
}
func TestLotterySecretNeverReturned(t *testing.T) {
	repo := &lotteryCaptchaRepoStub{config: LotteryConfig{TurnstileSecretKey: "private-secret", TurnstileSiteKey: "public-site"}}
	for _, userID := range []int64{0, 42} {
		snapshot, err := NewLotteryService(repo, nil, nil).Snapshot(context.Background(), userID)
		require.NoError(t, err)
		require.Empty(t, snapshot.Config.TurnstileSecretKey)
		require.True(t, snapshot.Config.TurnstileSecretConfigured)
		data, err := json.Marshal(snapshot)
		require.NoError(t, err)
		require.NotContains(t, string(data), "private-secret")
	}
}
