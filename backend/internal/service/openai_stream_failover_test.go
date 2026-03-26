package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOpenAIStreamingFailoverTestService() *OpenAIGatewayService {
	return &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				StreamDataIntervalTimeout: 0,
				StreamKeepaliveInterval:   0,
				MaxLineSize:               defaultMaxLineSize,
			},
		},
	}
}

func newOpenAIStreamingFailoverTestAccount() *Account {
	return &Account{
		ID:             101,
		Name:           "pool-openai",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeAPIKey,
		Schedulable:    true,
		Status:         StatusActive,
		RateMultiplier: f64p(1),
		Credentials: map[string]any{
			"pool_mode":             true,
			"pool_mode_retry_count": 3,
		},
	}
}

func newOpenAITransientErrorStreamResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"x-request-id": []string{"rid-stream-error"},
		},
		Body: ioNopCloserFromString(strings.Join([]string{
			`data: {"type":"error","error":{"type":"invalid_request_error","message":"An error occurred while processing your request. You can retry your request, or contact us through our help center at help.openai.com if the error persists. Please include the request ID req_123 in your message."}}`,
			"",
		}, "\n")),
	}
}

func ioNopCloserFromString(body string) *readCloserFromString {
	return &readCloserFromString{Reader: strings.NewReader(body)}
}

type readCloserFromString struct {
	*strings.Reader
}

func (r *readCloserFromString) Close() error { return nil }

func TestOpenAIStreamingErrorEventTriggersFailoverBeforeClientWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newOpenAIStreamingFailoverTestService()
	account := newOpenAIStreamingFailoverTestAccount()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	result, err := svc.handleStreamingResponse(c.Request.Context(), newOpenAITransientErrorStreamResponse(), c, account, time.Now(), "gpt-5.4", "gpt-5.4")
	require.NotNil(t, result)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Empty(t, rec.Body.String())
	require.Equal(t, -1, c.Writer.Size())
}

func TestOpenAIStreamingPassthroughErrorEventTriggersFailoverBeforeClientWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newOpenAIStreamingFailoverTestService()
	account := newOpenAIStreamingFailoverTestAccount()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), newOpenAITransientErrorStreamResponse(), c, account, time.Now())
	require.NotNil(t, result)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Empty(t, rec.Body.String())
	require.Equal(t, -1, c.Writer.Size())
}

func TestChatStreamingErrorEventTriggersFailoverBeforeClientWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newOpenAIStreamingFailoverTestService()
	account := newOpenAIStreamingFailoverTestAccount()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	result, err := svc.handleChatStreamingResponse(newOpenAITransientErrorStreamResponse(), c, account, "gpt-5.4", "gpt-5.4", false, time.Now())
	require.NotNil(t, result)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Empty(t, rec.Body.String())
	require.Equal(t, -1, c.Writer.Size())
}

func TestAnthropicStreamingErrorEventTriggersFailoverBeforeClientWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newOpenAIStreamingFailoverTestService()
	account := newOpenAIStreamingFailoverTestAccount()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	result, err := svc.handleAnthropicStreamingResponse(newOpenAITransientErrorStreamResponse(), c, account, "gpt-5.4", "gpt-5.4", time.Now())
	require.NotNil(t, result)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Empty(t, rec.Body.String())
	require.Equal(t, -1, c.Writer.Size())
}
