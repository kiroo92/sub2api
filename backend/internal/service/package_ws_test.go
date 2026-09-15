package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPackageWSHandoffPreservesVisibleHistory(t *testing.T) {
	h := &PackageWSHistory{}
	h.Begin([]byte(`{"type":"response.create","model":"gpt-5","input":[{"role":"user","content":"hello"}]}`))
	h.Complete(&OpenAIForwardResult{RequestID: "resp_1", wsAccountFailoverReplayInput: []json.RawMessage{json.RawMessage(`{"role":"assistant","content":"world"}`), json.RawMessage(`{"type":"function_call","call_id":"call_1","name":"read","arguments":"{}"}`)}})
	err := h.Handoff([]byte(`{"type":"response.create","previous_response_id":"resp_1","input":[{"type":"function_call_output","call_id":"call_1","output":"done"}]}`), "gpt-5", &APIKey{})
	var handoff *PackageWSHandoff
	require.ErrorAs(t, err, &handoff)
	require.False(t, gjson.GetBytes(handoff.Payload, "previous_response_id").Exists())
	require.Len(t, gjson.GetBytes(handoff.Payload, "input").Array(), 4)
	require.Contains(t, string(handoff.Payload), "world")
	require.Equal(t, "gpt-5", gjson.GetBytes(handoff.Payload, "model").String())
}

func TestPackageWSHandoffRejectsOpaqueOrMissingHistory(t *testing.T) {
	for _, body := range []string{
		`{"previous_response_id":"resp_unknown","input":[]}`,
		`{"input":[{"type":"function_call_output","call_id":"missing","output":"done"}]}`,
		`{"input":[{"type":"reasoning","encrypted_content":"opaque"}]}`,
	} {
		err := (&PackageWSHistory{}).Handoff([]byte(body), "gpt-5", &APIKey{})
		require.Error(t, err)
		var handoff *PackageWSHandoff
		require.NotErrorAs(t, err, &handoff)
	}
}
