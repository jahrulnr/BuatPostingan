package llm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/apperr"
)

// Catalog builds the model-picker list from configured providers (+ optional live enrich).
type Catalog struct {
	mu         sync.RWMutex
	providers  map[string]config.LLMProvider
	modelLists map[string][]string
	models     map[string][]config.LLMModel
	active     string
	effortCfg  string
	stub       bool
	effort     *EffortPolicy
	vision     *VisionPolicy
}

// NewCatalog wires picker listing. stub=true returns a canned list (no API keys exposed).
func NewCatalog(app config.Config, vision *VisionPolicy, effort *EffortPolicy) *Catalog {
	return &Catalog{
		providers:  app.LLMProviders,
		modelLists: app.LLMModelLists,
		models:     app.LLMModels,
		active:     app.LLMActiveProvider,
		effortCfg:  config.ParseEffortMode(app.LLMEffort),
		stub:       app.LLMStub,
		effort:     effort,
		vision:     vision,
	}
}

// Reload swaps runtime provider data after settings save.
func (c *Catalog) Reload(app config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers = app.LLMProviders
	c.modelLists = app.LLMModelLists
	c.models = app.LLMModels
	c.active = app.LLMActiveProvider
	c.effortCfg = config.ParseEffortMode(app.LLMEffort)
	c.stub = app.LLMStub
}

var _ service.ModelCatalog = (*Catalog)(nil)

func (c *Catalog) ListModels(ctx context.Context) (entity.ModelsCatalog, error) {
	c.mu.RLock()
	providers := c.providers
	modelLists := c.modelLists
	models := c.models
	active := c.active
	effortCfg := c.effortCfg
	stub := c.stub
	c.mu.RUnlock()

	out := entity.ModelsCatalog{
		EffortCurrent: effortCfg,
		EffortOptions: config.EffortPickerOptions(),
		Stub:          stub,
		Models:        nil,
	}
	if stub {
		out.Models = stubModels()
		out.DefaultModelID = out.Models[0].ID
		return out, nil
	}

	ids := make([]string, 0, len(providers))
	for id, p := range providers {
		if !p.Enabled {
			continue
		}
		if strings.TrimSpace(p.Model) == "" && strings.TrimSpace(p.APIKey) == "" && !p.APIKeyOptional && len(modelLists[id]) == 0 {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	seenModels := make(map[string]struct{}, len(providers))
	seenLabels := make(map[string]struct{}, len(providers))
	for _, id := range ids {
		p := providers[id]
		modelIDs := modelLists[id]
		if len(modelIDs) == 0 {
			modelID := strings.TrimSpace(p.Model)
			if modelID == "" {
				modelID = id
			}
			modelIDs = []string{modelID}
		}
		for _, modelID := range modelIDs {
			modelID = strings.TrimSpace(modelID)
			if modelID == "" {
				continue
			}
			meta, hasMeta := findModelMetadata(models[id], modelID)
			if !isChatSelectableModel(p, modelID, meta, hasMeta) {
				continue
			}
			if _, dup := seenModels[modelID]; dup {
				continue
			}
			label := modelLabel(modelID, id)
			if _, dup := seenLabels[label]; dup {
				continue
			}
			seenModels[modelID] = struct{}{}
			seenLabels[label] = struct{}{}
			opt := entity.ModelOption{
				ID:             modelID,
				Label:          modelLabel(modelID, id),
				Provider:       id,
				DefaultEffort:  "auto",
				SupportsVision: false,
			}
			if hasMeta {
				if meta.Label != "" {
					opt.Label = meta.Label
				}
				opt.Task = meta.Task
				opt.OutputModes = append([]string(nil), meta.OutputModes...)
			}
			if c.vision != nil {
				opt.SupportsVision = c.vision.SupportsImageFor(ctx, p)
			}
			if c.effort != nil {
				info := c.effort.lookup(ctx, p)
				if info.Supports {
					opt.SupportedEfforts = normalizeEffortList(info.SupportedEfforts)
					if len(opt.SupportedEfforts) == 0 {
						opt.SupportedEfforts = []string{
							EffortNone, EffortMinimal, EffortLow, EffortMedium,
							EffortHigh, EffortXHigh, EffortMax,
						}
					}
					def := strings.TrimSpace(info.DefaultEffort)
					if def != "" {
						opt.DefaultEffort = config.ParseEffortMode(def)
					} else {
						opt.DefaultEffort = EffortMedium
					}
				}
			} else if HeuristicModelSupportsEffort(modelID) {
				opt.SupportedEfforts = []string{
					EffortNone, EffortMinimal, EffortLow, EffortMedium,
					EffortHigh, EffortXHigh, EffortMax,
				}
				opt.DefaultEffort = EffortMedium
			}
			out.Models = append(out.Models, opt)
		}
	}

	if len(out.Models) == 0 {
		out.Models = stubModels()
		out.Stub = true
		out.DefaultModelID = out.Models[0].ID
		return out, nil
	}

	out.DefaultModelID = defaultModelID(active, providers, out.Models)
	return out, nil
}

func (c *Catalog) ResolveModel(_ context.Context, modelOrProvider string) (string, error) {
	raw := strings.TrimSpace(modelOrProvider)
	if raw == "" {
		return "", nil
	}
	c.mu.RLock()
	providers := c.providers
	modelLists := c.modelLists
	models := c.models
	stub := c.stub
	c.mu.RUnlock()

	if stub {
		for _, m := range stubModels() {
			if m.ID == raw || strings.EqualFold(m.Provider, raw) {
				return m.Provider, nil
			}
		}
		return "", apperr.Validation("model not allowed")
	}

	upper := strings.ToUpper(raw)
	if p, ok := providers[upper]; ok {
		if !p.Enabled {
			return "", apperr.Validation("model not allowed")
		}
		meta, found := findModelMetadata(models[p.ID], p.Model)
		if !isChatSelectableModel(p, p.Model, meta, found) {
			return "", apperr.Validation("model not allowed")
		}
		return p.ID, nil
	}
	for _, id := range sortedProviderIDs(providers) {
		p := providers[id]
		if !p.Enabled {
			continue
		}
		if strings.TrimSpace(p.Model) == raw {
			meta, found := findModelMetadata(models[id], raw)
			if !isChatSelectableModel(p, raw, meta, found) {
				return "", apperr.Validation("model not allowed")
			}
			return p.ID, nil
		}
		for _, mid := range modelLists[id] {
			if mid == raw {
				meta, found := findModelMetadata(models[id], mid)
				if !isChatSelectableModel(p, mid, meta, found) {
					return "", apperr.Validation("model not allowed")
				}
				return p.ID, nil
			}
		}
	}
	return "", apperr.Validation("model not allowed")
}

func findModelMetadata(models []config.LLMModel, modelID string) (config.LLMModel, bool) {
	for _, model := range models {
		if strings.TrimSpace(model.ID) == modelID {
			return model, true
		}
	}
	return config.LLMModel{}, false
}

// Empty output metadata is treated as legacy/unknown and remains selectable.
func supportsTextOutput(outputModes []string) bool {
	if len(outputModes) == 0 {
		return true
	}
	for _, mode := range outputModes {
		if strings.EqualFold(strings.TrimSpace(mode), "text") {
			return true
		}
	}
	return false
}

func isChatSelectableModel(provider config.LLMProvider, modelID string, meta config.LLMModel, hasMeta bool) bool {
	if hasMeta {
		if !supportsTextOutput(meta.OutputModes) || isKnownNonChatTask(meta.Task) {
			return false
		}
	}
	return !isOpenAIProtocolProvider(provider) || !isKnownNonChatModelID(modelID)
}

func isOpenAIProtocolProvider(provider config.LLMProvider) bool {
	switch strings.ToLower(strings.TrimSpace(provider.Type)) {
	case "openai", "openai-compatible", "openrouter", "omniroute", "9router":
		return true
	case "claude":
		return false
	}
	api := strings.ToLower(strings.TrimSpace(provider.API))
	return api == "chat" || api == "responses"
}

func isKnownNonChatTask(task string) bool {
	switch strings.ToLower(strings.TrimSpace(task)) {
	case "embedding", "embeddings",
		"text-to-speech", "speech-to-text", "tts", "stt",
		"transcription", "translation",
		"image-generation", "video-generation",
		"moderation", "rerank", "reranking",
		"classification", "ocr":
		return true
	default:
		return false
	}
}

func isKnownNonChatModelID(modelID string) bool {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if id == "" {
		return false
	}
	markers := []string{
		"embedding", "embed-", "-embed", "/embed",
		"rerank", "re-rank", "moderation",
		"transcribe", "transcription", "whisper",
		"text-to-speech", "speech-to-text", "-tts", "/tts", "tts-", "-stt", "/stt", "stt-",
		"gpt-image", "dall-e", "image-generation", "imagegen",
		"stable-diffusion", "sdxl", "seedream",
		"video-generation",
	}
	for _, marker := range markers {
		if strings.Contains(id, marker) {
			return true
		}
	}
	return hasModelFamilyPrefix(id, "sora") ||
		hasModelFamilyPrefix(id, "flux") ||
		hasModelFamilyPrefix(id, "imagen") ||
		hasModelFamilyPrefix(id, "veo")
}

func hasModelFamilyPrefix(modelID, family string) bool {
	return modelID == family ||
		strings.HasPrefix(modelID, family+"-") ||
		strings.Contains(modelID, "/"+family+"-")
}

func stubModels() []entity.ModelOption {
	return []entity.ModelOption{
		{
			ID:             "stub/default",
			Label:          "Stub default",
			Provider:       "STUB",
			SupportsVision: false,
			DefaultEffort:  "auto",
		},
		{
			ID:               "stub/reasoning",
			Label:            "Stub reasoning",
			Provider:         "STUB",
			SupportsVision:   false,
			SupportedEfforts: []string{EffortNone, EffortLow, EffortMedium, EffortHigh},
			DefaultEffort:    EffortMedium,
		},
		{
			ID:             "stub/vision",
			Label:          "Stub vision",
			Provider:       "STUB",
			SupportsVision: true,
			DefaultEffort:  "auto",
		},
	}
}

func modelLabel(modelID, providerID string) string {
	if modelID == "" || modelID == providerID {
		return providerID
	}
	return fmt.Sprintf("%s · %s", modelID, providerID)
}

func defaultModelID(active string, providers map[string]config.LLMProvider, models []entity.ModelOption) string {
	if active != "" {
		if p, ok := providers[active]; ok && p.Enabled {
			m := strings.TrimSpace(p.Model)
			if m == "" {
				m = p.ID
			}
			for _, opt := range models {
				if opt.ID == m && opt.Provider == p.ID {
					return opt.ID
				}
			}
			for _, opt := range models {
				if opt.ID == m {
					return opt.ID
				}
			}
		}
	}
	if len(models) > 0 {
		return models[0].ID
	}
	return ""
}

func sortedProviderIDs(providers map[string]config.LLMProvider) []string {
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func normalizeEffortList(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, e := range raw {
		n, ok := config.NormalizeEffortOverride(e)
		if !ok || n == "" || n == EffortAuto {
			continue
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}
