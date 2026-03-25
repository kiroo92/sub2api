package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAICompactRequestBodyPreservesConversationState(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5.4",
		"conversation":"conv_compact_123",
		"prompt_cache_key":"pcache_compact_123",
		"instructions":"compact this conversation",
		"metadata":{"ignored":true}
	}`)

	normalized, changed, err := NormalizeOpenAICompactRequestBodyForTest(body)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(normalized, "model").String())
	require.Equal(t, "conv_compact_123", gjson.GetBytes(normalized, "conversation").String())
	require.Equal(t, "pcache_compact_123", gjson.GetBytes(normalized, "prompt_cache_key").String())
	require.Equal(t, "compact this conversation", gjson.GetBytes(normalized, "instructions").String())
	require.False(t, gjson.GetBytes(normalized, "metadata").Exists())
}
