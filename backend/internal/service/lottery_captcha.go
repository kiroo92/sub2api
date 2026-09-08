package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// VerifyLotteryCaptcha requires a server-verified action challenge even when login CAPTCHA is optional.
func (s *AuthService) VerifyLotteryCaptcha(ctx context.Context, proof CaptchaProof, remoteIP string) error {
	if s == nil || s.settingService == nil {
		return ErrServiceUnavailable
	}

	providerConfig, err := s.settingService.GetCaptchaProviderConfig(ctx)
	if err != nil {
		logger.LegacyPrintf("service.auth", "%s", "[Auth] Failed to read captcha provider settings")
		return ErrServiceUnavailable
	}
	tencentEnabled := providerConfig.Tencent.Enabled
	aliyunEnabled := providerConfig.Aliyun.Enabled
	if !tencentEnabled && !aliyunEnabled {
		return infraerrors.ServiceUnavailable("LOTTERY_CAPTCHA_NOT_CONFIGURED", "请在后台配置腾讯或阿里云滑动验证后参与抽奖")
	}
	if captchaProvidersConflict(providerConfig.TurnstileEnabled, tencentEnabled, aliyunEnabled) {
		return ErrCaptchaProviderConflict
	}
	if aliyunEnabled {
		if s.aliyunCaptchaService == nil {
			return ErrAliyunCaptchaNotConfigured
		}
		return s.aliyunCaptchaService.VerifyParamWithConfig(ctx, providerConfig.Aliyun, proof.TurnstileToken)
	}
	if s.tencentCaptchaService == nil {
		return ErrTencentCaptchaNotConfigured
	}
	return s.tencentCaptchaService.VerifyTicketWithConfig(
		ctx,
		providerConfig.Tencent,
		proof.TencentTicket,
		proof.TencentRandstr,
		remoteIP,
	)
}
