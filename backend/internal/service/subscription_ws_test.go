package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSubscriptionWSReplayPreservesHistoryAndRejectsUnknownTools(t *testing.T) {
	payload := []byte(`{"type":"response.create","model":"gpt-5","previous_response_id":"resp_first","input":[{"type":"function_call_output","call_id":"call_1","output":"done"}]}`)
	history := []json.RawMessage{
		json.RawMessage(`{"role":"user","content":"first"}`),
		json.RawMessage(`{"type":"function_call","call_id":"call_1","name":"inspect","arguments":"{}"}`),
	}
	err := subscriptionWSReplayError(ErrSubscriptionWSReselect, payload, history, true, "gpt-5")
	require.ErrorIs(t, err, ErrSubscriptionWSReselect)
	retry, ok := OpenAIWSCurrentTurnRetryPayload(err)
	require.True(t, ok)
	require.False(t, gjson.GetBytes(retry, "previous_response_id").Exists())
	require.Len(t, gjson.GetBytes(retry, "input").Array(), 3)
	err = subscriptionWSReplayError(ErrSubscriptionWSReselect, payload, nil, false, "gpt-5")
	_, ok = OpenAIWSCurrentTurnRetryPayload(err)
	require.False(t, ok)
	var closeErr *OpenAIWSClientCloseError
	require.ErrorAs(t, err, &closeErr)
}

func TestSubscriptionWSNativeAndBridgeSwitchKeepsClientConnected(t *testing.T) {
	for _, mode := range []string{OpenAIWSIngressModeCtxPool, OpenAIWSIngressModeHTTPBridge} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), ctxkey.AllSubscriptions, true)
			cfg := passthroughLifecycleConfig()
			cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
			cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
			cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
			firstEvent := `{"type":"response.completed","response":{"id":"resp_first","model":"gpt-5.1","output":[{"id":"msg_first","type":"message","role":"assistant","content":[{"type":"output_text","text":"first reply"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}`
			capture := &openAIWSCaptureConn{events: [][]byte{[]byte(firstEvent)}}
			pool := newOpenAIWSConnPool(cfg)
			pool.setClientDialerForTest(&openAIWSCaptureDialer{conn: capture})
			svc := &OpenAIGatewayService{cfg: cfg, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), openaiWSPool: pool, toolCorrector: NewCodexToolCorrector(), httpUpstream: &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: " + firstEvent + "\n\n"))}}}
			account := passthroughLifecycleAccount()
			account.Extra["openai_apikey_responses_websockets_v2_mode"] = mode
			result := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					result <- err
					return
				}
				defer func() { _ = conn.CloseNow() }()
				_, first, err := conn.Read(ctx)
				if err != nil {
					result <- err
					return
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = r.Clone(ctx)
				err = svc.ProxyResponsesWebSocketFromClient(ctx, c, conn, account, "sk-test", first, &OpenAIWSIngressHooks{BeforeRequest: func(int, []byte, string) error { return ErrSubscriptionWSReselect }})
				payload, ok := OpenAIWSCurrentTurnRetryPayload(err)
				if !ok || !strings.Contains(string(payload), "first reply") || gjson.GetBytes(payload, "previous_response_id").Exists() {
					result <- errors.New("missing complete handoff history: " + string(payload))
					return
				}
				// The caller can now choose a different group on the same client connection.
				result <- conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_second"}}`))
			}))
			defer server.Close()
			client := dialPassthroughLifecycleClientWithPayload(t, server, `{"type":"response.create","model":"gpt-5.1","input":[{"role":"user","content":"first"}]}`)
			defer func() { _ = client.CloseNow() }()
			_, err := readPassthroughLifecycleFrame(t, client, 4*time.Second)
			require.NoError(t, err)
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`)))
			frame, err := readPassthroughLifecycleFrame(t, client, 4*time.Second)
			require.NoError(t, err)
			require.Equal(t, "resp_second", gjson.GetBytes(frame, "response.id").String())
			require.NoError(t, <-result)
		})
	}
}

func TestSubscriptionWSPassthroughReturnsUnsentTurnWithHistory(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxkey.AllSubscriptions, true)
	upstream := newStagedPassthroughConn()
	upstream.Send(`{"type":"response.completed","response":{"id":"resp_first","model":"gpt-5.1","output":[{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"first reply"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}`)
	hooks := &OpenAIWSIngressHooks{BeforeRequest: func(int, []byte, string) error { return ErrSubscriptionWSReselect }}
	server, result := startPassthroughHookRecordingServer(t, ctx, newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream), passthroughLifecycleAccount(), hooks)
	defer server.Close()
	client := dialPassthroughLifecycleClientWithPayload(t, server, `{"type":"response.create","model":"gpt-5.1","input":[{"role":"user","content":"first"}]}`)
	defer func() { _ = client.CloseNow() }()
	requirePassthroughUpstreamWrite(t, upstream, time.Second)
	_, err := readPassthroughLifecycleFrame(t, client, 3*time.Second)
	require.NoError(t, err)
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`)))
	select {
	case err := <-result:
		require.True(t, errors.Is(err, ErrSubscriptionWSReselect), "%v", err)
		payload, ok := OpenAIWSCurrentTurnRetryPayload(err)
		require.True(t, ok)
		require.Contains(t, string(payload), "first reply")
		require.Contains(t, string(payload), "second")
		require.False(t, gjson.GetBytes(payload, "previous_response_id").Exists())
	case <-time.After(5 * time.Second):
		t.Fatal("subscription handoff did not finish")
	}
	select {
	case payload := <-upstream.writes:
		t.Fatalf("unadmitted turn sent to old upstream: %s", payload)
	default:
	}
}
