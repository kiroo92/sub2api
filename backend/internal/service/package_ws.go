package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"sync"

	"github.com/tidwall/gjson"
)

type PackageWSHandoff struct {
	Key     *APIKey
	Payload []byte
	Model   string
}

func (e *PackageWSHandoff) Error() string {
	return "package websocket route changed before sending next turn"
}

// PackageWSHistory retains only client-visible history, never upstream opaque state.
// ponytail: 16 MiB transcript ceiling; explicit resend/full-history required above it.
type PackageWSHistory struct {
	mu           sync.Mutex
	items        []json.RawMessage
	pending      []json.RawMessage
	lastResponse string
	portable     bool
}

func (h *PackageWSHistory) Begin(payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pending, h.portable = h.input(payload)
}

func (h *PackageWSHistory) input(payload []byte) ([]json.RawMessage, bool) {
	var envelope struct {
		Input    json.RawMessage `json:"input"`
		Previous string          `json:"previous_response_id"`
	}
	if json.Unmarshal(payload, &envelope) != nil {
		return nil, false
	}
	var items []json.RawMessage
	if len(envelope.Input) > 0 && envelope.Input[0] == '"' {
		var text string
		_ = json.Unmarshal(envelope.Input, &text)
		b, _ := json.Marshal(map[string]string{"role": "user", "content": text})
		items = []json.RawMessage{b}
	} else if len(envelope.Input) > 0 && json.Unmarshal(envelope.Input, &items) != nil {
		return nil, false
	}
	if envelope.Previous != "" {
		if !h.portable || h.lastResponse == "" || envelope.Previous != h.lastResponse {
			return items, false
		}
		items = combineOpenAIWSReplayItems(h.items, items)
	}
	return items, !bytes.Contains(payload, []byte(`"encrypted_content"`))
}

func (h *PackageWSHistory) Complete(result *OpenAIForwardResult) {
	if result == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = combineOpenAIWSReplayItems(h.pending, result.wsAccountFailoverReplayInput)
	h.lastResponse = result.RequestID
	size := 0
	for _, item := range h.items {
		size += len(item)
		if bytes.Contains(item, []byte(`"encrypted_content"`)) {
			h.portable = false
		}
	}
	if size > 16<<20 {
		h.items = nil
		h.portable = false
	}
}

func (h *PackageWSHistory) Handoff(payload []byte, model string, key *APIKey) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	items, portable := h.input(payload)
	if !portable {
		return errors.New("package_context_required: resend complete visible history to switch package groups")
	}
	full, ok, err := buildOpenAIWSCurrentTurnRetryPayload(payload, items, true, model)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("package_context_required: missing tool call history")
	}
	h.pending, h.portable = items, true
	return &PackageWSHandoff{Key: key, Payload: full, Model: model}
}

func packageWSModel(payload []byte, fallback string) string {
	if model := gjson.GetBytes(payload, "model").String(); model != "" {
		return model
	}
	return fallback
}
