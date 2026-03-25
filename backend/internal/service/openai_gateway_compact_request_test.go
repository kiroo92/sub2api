package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAICompactRequestBodyKeepsOnlyOfficialCompactFields(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.4",
		"input":[{"type":"input_text","text":"compact this conversation"}],
		"instructions":"compact this conversation",
		"previous_response_id":"resp_compact_123",
		"conversation":"conv_compact_123",
		"prompt_cache_key":"pcache_compact_123",
		"metadata":{"ignored":true}
	}`)

	normalized, changed, err := NormalizeOpenAICompactRequestBodyForTest(body)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "compact this conversation", gjson.GetBytes(normalized, "input.0.text").String())
	require.Equal(t, "compact this conversation", gjson.GetBytes(normalized, "instructions").String())
	require.Equal(t, "resp_compact_123", gjson.GetBytes(normalized, "previous_response_id").String())
	require.Equal(t, "pcache_compact_123", gjson.GetBytes(normalized, "prompt_cache_key").String())
	require.False(t, gjson.GetBytes(normalized, "conversation").Exists())
	require.False(t, gjson.GetBytes(normalized, "metadata").Exists())
}
