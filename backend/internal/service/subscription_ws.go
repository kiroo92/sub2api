package service

import (
	"encoding/json"
	"errors"

	coderws "github.com/coder/websocket"
)

// ErrSubscriptionWSReselect is returned before writing an unexecuted turn.
var ErrSubscriptionWSReselect = errors.New("subscription websocket upstream must change")

func subscriptionWSReplayError(cause error, payload []byte, history []json.RawMessage, historyExists bool, model string) error {
	if !errors.Is(cause, ErrSubscriptionWSReselect) {
		return cause
	}
	previous := openAIWSPayloadStringFromRaw(payload, "previous_response_id")
	if previous != "" && !historyExists {
		return NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "subscription switch needs complete conversation history", cause)
	}
	items, exists, err := buildOpenAIWSReplayInputSequence(history, historyExists, payload, previous != "" || openAIWSRawPayloadHasToolCallOutput(payload))
	if err != nil {
		return err
	}
	retry, safe, err := buildOpenAIWSCurrentTurnRetryPayload(payload, items, exists, model)
	if err != nil {
		return err
	}
	if !safe {
		return NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "subscription switch needs complete tool call history", cause)
	}
	return newOpenAIWSCurrentTurnFailoverError(ErrSubscriptionWSReselect, retry)
}
