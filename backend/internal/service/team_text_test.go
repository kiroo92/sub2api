package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTeamTextOnlyCapabilities(t *testing.T) {
	for _, path := range []string{"/v1/images/generations", "/v1/images/edits/async", "/v1/images/tasks/id", "/v1/videos", "/v1/live", "/realtime", "/images/batches"} {
		require.ErrorIs(t, ValidateTeamTextRequest(path, nil, nil), ErrTeamTextOnly)
	}
	for _, body := range []string{
		`{"model":"gpt-image-1"}`, `{"model":"gemini-3-pro-image-preview"}`,
		`{"tools":[{"type":"image_generation"}]}`, `{"tools":[],"tools":[{"type":"image_generation"}]}`,
		`{"tool_choice":{"type":"image_generation"}}`, `{"generationConfig":{"responseModalities":["TEXT","IMAGE"]}}`,
		`{"type":"response.create","response":{"tools":[{"type":"image_generation"}]}}`,
	} {
		require.ErrorIs(t, ValidateTeamTextRequest("/v1/responses", nil, []byte(body)), ErrTeamTextOnly, body)
	}
	require.NoError(t, ValidateTeamTextRequest("/v1/responses", []string{"gpt-5"}, []byte(`{"input":[{"type":"input_image","image_url":"https://example.com/a.png"},{"type":"input_text","text":"Describe this image"}]}`)))
	// Both HTTP selection and each WebSocket turn use this service entry point.
	svc := &SubscriptionService{}
	_, _, err := svc.SelectForRequest(context.Background(), &APIKey{RoutingMode: "team"}, SubscriptionRequest{Path: "/v1/responses", WebSocket: true, Body: []byte(`{"tools":[{"type":"image_generation"}]}`)})
	require.ErrorIs(t, err, ErrTeamTextOnly)
}
