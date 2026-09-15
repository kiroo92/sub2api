package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *GatewayHandler) packageModels(c *gin.Context, key *service.APIKey, codex bool) {
	groups, err := h.apiKeyService.PackageGroups(c.Request.Context(), key)
	if err != nil {
		h.errorResponse(c, 503, "api_error", "Package model discovery is unavailable")
		return
	}
	forced, _ := middleware.GetForcePlatformFromContext(c)
	if strings.HasPrefix(c.Request.URL.Path, "/antigravity/") {
		forced = service.PlatformAntigravity
	}
	entries := make([]json.RawMessage, 0)
	seen := make(map[string]bool)
	var bodies [][]byte
	for _, group := range groups {
		if forced != "" && group.Platform != forced && group.Platform != service.PlatformComposite {
			continue
		}
		models := h.codexModelIDsForGroup(c.Request.Context(), group, forced)
		if codex {
			var body []byte
			if group.Platform == service.PlatformOpenAI && forced == "" {
				body, err = h.packageOpenAICodexBody(c, group)
			} else {
				body, err = h.gatewayService.BuildCodexModelsManifestForGroup(c.Request.Context(), group, forced, service.FilterCodexModelIDsForGroup(models, group))
			}
			if err != nil {
				h.errorResponse(c, 503, "api_error", "Package model discovery is unavailable")
				return
			}
			bodies = append(bodies, body)
			continue
		}
		if group.Platform == service.PlatformOpenAI && group.CodexModelsManifestConfig.Enabled && forced == "" {
			response, _, e := h.openAIGatewayService.FetchPinnedOpenAIModelsList(c.Request.Context(), group, h.maxAccountSwitches, "")
			if e != nil {
				h.errorResponse(c, 503, "api_error", "Package model discovery is unavailable")
				return
			}
			var envelope struct {
				Data []json.RawMessage `json:"data"`
			}
			if json.Unmarshal(response.Body, &envelope) != nil {
				h.errorResponse(c, 503, "api_error", "Invalid upstream model list")
				return
			}
			for _, item := range envelope.Data {
				var header struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(item, &header) != nil || header.ID == "" {
					continue
				}
				if group.ModelAllowlist.Allows(header.ID) && !seen[header.ID] {
					seen[header.ID] = true
					entries = append(entries, item)
				}
			}
			continue
		}
		for _, item := range openAIModelsForIDs(models) {
			if group.ModelAllowlist.Allows(item.ID) && !seen[item.ID] {
				seen[item.ID] = true
				raw, _ := json.Marshal(item)
				entries = append(entries, raw)
			}
		}
	}
	if codex {
		body, err := service.MergePackageCodexModelsManifests(bodies)
		if err != nil {
			h.errorResponse(c, 503, "api_error", "Unable to merge model lists")
			return
		}
		etag := service.CodexModelsManifestETag(body)
		c.Header("ETag", etag)
		if service.CodexModelsManifestETagMatches(c.GetHeader("If-None-Match"), etag) {
			c.Status(http.StatusNotModified)
			c.Writer.WriteHeaderNow()
			return
		}
		c.Data(http.StatusOK, "application/json", body)
		return
	}
	body, err := json.Marshal(gin.H{"object": "list", "data": entries})
	if err != nil {
		h.errorResponse(c, 503, "api_error", "Unable to merge model lists")
		return
	}
	etag := service.CodexModelsManifestETag(body)
	c.Header("ETag", etag)
	if service.CodexModelsManifestETagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		c.Writer.WriteHeaderNow()
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}

func (h *GatewayHandler) packageOpenAICodexBody(c *gin.Context, group *service.Group) ([]byte, error) {
	s := h.openAIGatewayService
	if s == nil {
		return nil, fmt.Errorf("OpenAI discovery unavailable")
	}
	var manifest *service.OpenAIModelsResponse
	var err error
	if group.CodexModelsManifestConfig.Enabled {
		manifest, _, err = s.FetchPinnedCodexModelsManifest(c.Request.Context(), group, c.Query("client_version"))
		if err != nil && !group.CodexModelsManifestConfig.FallbackToScheduler {
			return nil, err
		}
	} else {
		var configured bool
		manifest, configured, err = s.BuildGroupConfiguredCodexModelsManifest(c.Request.Context(), group, "")
		if err != nil {
			return nil, err
		}
		if configured {
			return manifest.Body, nil
		}
		manifest = nil
	}
	if manifest == nil {
		account, e := s.SelectAccountForModelWithExclusions(c.Request.Context(), &group.ID, "", "", nil)
		if e != nil {
			return nil, e
		}
		manifest, err = s.FetchCodexModelsManifest(c.Request.Context(), account, c.Query("client_version"), "")
		if err != nil {
			return nil, err
		}
		if err = s.CompleteAPIKeyCodexModelsManifestForClient(manifest, account); err != nil {
			return nil, err
		}
		if err = service.ApplyPinnedCodexModelsMapping(manifest, account, group); err != nil {
			return nil, err
		}
	}
	if err = s.MergeGroupConfiguredCodexModels(c.Request.Context(), group, manifest, ""); err != nil {
		return nil, err
	}
	return manifest.Body, nil
}

// Gemini's native protocol needs one merged pagination domain, not the first
// group's opaque upstream token. Snapshot-derived tokens bind the merged list.
func (h *GatewayHandler) packageGeminiModels(c *gin.Context, key *service.APIKey, single bool) {
	groups, err := h.apiKeyService.PackageGroups(c.Request.Context(), key)
	if err != nil {
		googleError(c, 503, "Package model discovery is unavailable")
		return
	}
	models := make([]json.RawMessage, 0)
	seen := make(map[string]bool)
	for _, g := range groups {
		if g.Platform != service.PlatformGemini && g.Platform != service.PlatformAntigravity && g.Platform != service.PlatformComposite {
			continue
		}
		var items []json.RawMessage
		if g.Platform == service.PlatformAntigravity {
			for _, m := range gemini.DefaultModels() {
				b, _ := json.Marshal(m)
				items = append(items, b)
			}
		} else {
			account, e := h.geminiCompatService.SelectAccountForAIStudioEndpoints(c.Request.Context(), &g.ID)
			if e != nil {
				googleError(c, 503, "Package model discovery is unavailable")
				return
			}
			next, seenPages := "", map[string]bool{}
			for {
				res, e := h.geminiCompatService.ForwardAIStudioModelsPage(c.Request.Context(), account, next)
				if e != nil {
					googleError(c, 503, "Package model discovery is unavailable")
					return
				}
				if shouldFallbackGeminiModels(res) {
					for _, m := range gemini.DefaultModels() {
						b, _ := json.Marshal(m)
						items = append(items, b)
					}
					break
				} else {
					var envelope struct {
						Models        []json.RawMessage `json:"models"`
						NextPageToken string            `json:"nextPageToken"`
					}
					if json.Unmarshal(res.Body, &envelope) != nil {
						googleError(c, 503, "Complete upstream model list is required")
						return
					}
					items = append(items, envelope.Models...)
					if envelope.NextPageToken == "" {
						break
					}
					if seenPages[envelope.NextPageToken] || len(seenPages) >= 100 {
						googleError(c, 503, "Invalid upstream model pagination")
						return
					}
					seenPages[envelope.NextPageToken] = true
					next = envelope.NextPageToken
				}
			}
		}
		for _, raw := range items {
			var m struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(raw, &m) != nil || m.Name == "" {
				continue
			}
			if g.ModelAllowlist.Allows(m.Name) && !seen[m.Name] {
				seen[m.Name] = true
				models = append(models, raw)
			}
		}
	}
	if single {
		name := "models/" + strings.TrimPrefix(c.Param("model"), "models/")
		for _, raw := range models {
			var m struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(raw, &m)
			if m.Name == name {
				c.Data(200, "application/json", raw)
				return
			}
		}
		googleError(c, 404, "Model is not available for these packages")
		return
	}
	all, _ := json.Marshal(models)
	etag := service.CodexModelsManifestETag(all)
	start := 0
	if token := c.Query("pageToken"); token != "" {
		parts := strings.SplitN(token, ":", 2)
		if len(parts) != 2 || parts[0] != strings.Trim(etag, "\"") {
			googleError(c, 400, "Model list changed; restart pagination")
			return
		}
		start, err = strconv.Atoi(parts[1])
		if err != nil || start < 0 || start > len(models) {
			googleError(c, 400, "Invalid page token")
			return
		}
	}
	size := 1000
	if q := c.Query("pageSize"); q != "" {
		size, err = strconv.Atoi(q)
		if err != nil || size < 1 || size > 1000 {
			googleError(c, 400, "pageSize must be between 1 and 1000")
			return
		}
	}
	end := min(start+size, len(models))
	body := gin.H{"models": models[start:end]}
	if end < len(models) {
		body["nextPageToken"] = strings.Trim(etag, "\"") + ":" + strconv.Itoa(end)
	}
	c.JSON(200, body)
}
