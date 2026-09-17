package service

import (
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/tidwall/gjson"
	"strings"
)

var ErrTeamTextOnly = infraerrors.BadRequest("TEAM_ENDPOINT_UNSUPPORTED", "Team keys support text requests only; image generation, video and Live are not supported")

// ValidateTeamTextRequest checks capabilities, not prompt prose or image inputs.
func ValidateTeamTextRequest(path string, models []string, body []byte) error {
	for _, part := range []string{"/images", "/videos", "/live", "/realtime", "/batches", "/audio"} {
		if strings.Contains(path, part) {
			return ErrTeamTextOnly
		}
	}
	for _, model := range models {
		if teamGenerationModel(model) {
			return ErrTeamTextOnly
		}
	}
	var forbidden func(gjson.Result, int) bool
	forbidden = func(node gjson.Result, depth int) bool {
		if depth > 128 {
			return true
		}
		blocked := false
		node.ForEach(func(key, value gjson.Result) bool {
			switch strings.ToLower(key.String()) {
			case "model":
				blocked = teamGenerationModel(value.String())
			case "type", "name":
				v := strings.ToLower(value.String())
				blocked = v == "image_generation" || v == "image_gen"
			case "responsemodalities", "response_modalities", "modalities":
				value.ForEach(func(_, v gjson.Result) bool {
					blocked = strings.EqualFold(v.String(), "IMAGE") || strings.EqualFold(v.String(), "AUDIO")
					return !blocked
				})
			}
			if !blocked && (value.IsObject() || value.IsArray()) {
				blocked = forbidden(value, depth+1)
			}
			return !blocked
		})
		return blocked
	}
	if len(body) > 0 && gjson.ValidBytes(body) && forbidden(gjson.ParseBytes(body), 0) {
		return ErrTeamTextOnly
	}
	return nil
}
func teamGenerationModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return IsImageGenerationIntent("", model, nil) || strings.Contains(model, "-image") || strings.HasPrefix(model, "imagen") || strings.Contains(model, "grok-imagine")
}
