//go:build unit

package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLotteryCaptchaRequired(t *testing.T) {
	svc := newAuthServiceForCaptchaTest(map[string]string{}, false, nil, nil)
	require.Error(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{}, "127.0.0.1"))
	verifier := &tencentCaptchaVerifierStub{response: &TencentCaptchaVerifyResponse{CaptchaCode: 1}}
	svc = newAuthServiceForCaptchaTest(tencentCaptchaSettings(), false, nil, verifier)
	require.ErrorIs(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{}, "127.0.0.1"), ErrTencentCaptchaVerificationFailed)
	require.Zero(t, verifier.calls)
	require.NoError(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{TencentTicket: "ticket", TencentRandstr: "rand"}, "127.0.0.1"))
	require.Equal(t, 1, verifier.calls)
	verifier.response.CaptchaCode = 0
	require.ErrorIs(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{TencentTicket: "expired", TencentRandstr: "rand"}, "127.0.0.1"), ErrTencentCaptchaVerificationFailed)
}
func TestLotteryCaptchaAliyun(t *testing.T) {
	spy := &aliyunVerifierSpy{}
	svc := newAliyunAuthServiceForTest(&config.Config{}, aliyunEnabledSettings(), spy)
	require.NoError(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{TurnstileToken: "aliyun-proof"}, "127.0.0.1"))
	require.Equal(t, "aliyun-proof", spy.lastParam)
	require.Error(t, svc.VerifyLotteryCaptcha(context.Background(), CaptchaProof{}, "127.0.0.1"))
}
