package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"strings"
)

// LotteryCaptchaService uses only lottery credentials, independent of login settings.
type LotteryCaptchaService struct {
	repo     LotteryRepository
	verifier TurnstileVerifier
}

func NewLotteryCaptchaService(repo LotteryRepository, verifier TurnstileVerifier) *LotteryCaptchaService {
	return &LotteryCaptchaService{repo: repo, verifier: verifier}
}
func (s *LotteryCaptchaService) VerifyLotteryCaptcha(ctx context.Context, proof CaptchaProof, remoteIP string) error {
	if s == nil || s.repo == nil || s.verifier == nil {
		return ErrServiceUnavailable
	}
	snapshot, err := s.repo.Snapshot(ctx, 0)
	if err != nil {
		return ErrServiceUnavailable
	}
	if snapshot.Config.TurnstileSiteKey == "" || snapshot.Config.TurnstileSecretKey == "" {
		return infraerrors.ServiceUnavailable("LOTTERY_CAPTCHA_NOT_CONFIGURED", "请配置抽奖专用 Turnstile")
	}
	if strings.TrimSpace(proof.TurnstileToken) == "" {
		return ErrTurnstileVerificationFailed
	}
	result, err := s.verifier.VerifyToken(ctx, snapshot.Config.TurnstileSecretKey, proof.TurnstileToken, remoteIP)
	if err != nil {
		return ErrServiceUnavailable
	}
	if result == nil || !result.Success {
		return ErrTurnstileVerificationFailed
	}
	return nil
}
