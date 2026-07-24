package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"buatpostingan/internal/config"
)

// Vision modes for BP_LLM_VISION.
const (
	VisionAuto = "auto"
	VisionOn   = "on"
	VisionOff  = "off"
)

// VisionPolicy decides whether the worker may attach image pixels to LLM user content.
type VisionPolicy struct {
	mode      string
	providers map[string]config.LLMProvider
	active    string
	http      *http.Client

	mu    sync.Mutex
	cache map[string]bool // key: baseURL + "\x00" + modelID
}

// NewVisionPolicy builds a gate from app LLM config.
func NewVisionPolicy(cfg Config) *VisionPolicy {
	mode := config.ParseVisionMode(cfg.Vision)
	return &VisionPolicy{
		mode:      mode,
		providers: cfg.Providers,
		active:    cfg.ActiveProvider,
		http:      &http.Client{Timeout: 12 * time.Second},
		cache:     map[string]bool{},
	}
}

// AllowPixels reports whether multimodal image parts should be injected.
func (p *VisionPolicy) AllowPixels(ctx context.Context) bool {
	if p == nil {
		return true
	}
	switch p.mode {
	case VisionOff:
		return false
	case VisionOn:
		return true
	default: // auto
		prov, model, base, key := p.activeSlot()
		if model == "" {
			return false
		}
		return p.modelSupportsImage(ctx, prov, base, key, model)
	}
}

// SupportsImageFor reports whether a configured provider/model advertises vision
// (catalog probe or heuristic). Used by the model picker; ignores BP_LLM_VISION=off/on
// force modes so the UI can still show capability chips.
func (p *VisionPolicy) SupportsImageFor(ctx context.Context, prov config.LLMProvider) bool {
	if p == nil {
		return HeuristicModelSupportsImage(prov.Model)
	}
	model := strings.TrimSpace(prov.Model)
	if model == "" {
		return false
	}
	base := strings.TrimRight(strings.TrimSpace(prov.BaseURL), "/")
	key := strings.TrimSpace(prov.APIKey)
	return p.modelSupportsImage(ctx, prov.ID, base, key, model)
}

// Mode returns the normalized vision mode.
func (p *VisionPolicy) Mode() string {
	if p == nil {
		return VisionAuto
	}
	return p.mode
}

func (p *VisionPolicy) activeSlot() (id, model, baseURL, apiKey string) {
	id = p.active
	if id == "" {
		return "", "", "", ""
	}
	prov, ok := p.providers[id]
	if !ok {
		return id, "", "", ""
	}
	return prov.ID, strings.TrimSpace(prov.Model), strings.TrimRight(strings.TrimSpace(prov.BaseURL), "/"), strings.TrimSpace(prov.APIKey)
}

func (p *VisionPolicy) modelSupportsImage(ctx context.Context, providerID, baseURL, apiKey, modelID string) bool {
	cacheKey := baseURL + "\x00" + modelID
	p.mu.Lock()
	if v, ok := p.cache[cacheKey]; ok {
		p.mu.Unlock()
		return v
	}
	p.mu.Unlock()

	supported := false
	probed := false
	if baseURL != "" {
		if ok, hit := p.probeModelsAPI(ctx, baseURL, apiKey, modelID); hit {
			supported = ok
			probed = true
		}
	}
	if !probed {
		supported = HeuristicModelSupportsImage(modelID)
	}

	p.mu.Lock()
	p.cache[cacheKey] = supported
	p.mu.Unlock()
	_ = providerID
	return supported
}

// HeuristicModelSupportsImage is a best-effort local check when Models API is unavailable.
func HeuristicModelSupportsImage(modelID string) bool {
	s := strings.ToLower(strings.TrimSpace(modelID))
	if s == "" {
		return false
	}
	// Explicit vision / omni markers.
	positive := []string{
		"mimo", "omni", "vision", "-vl", "vl-", "pixtral", "llava",
		"gpt-4o", "gpt-4.1", "gpt-4-turbo", "gpt-4-vision", "gpt-5",
		"claude-3", "claude-4", "claude-sonnet", "claude-opus", "claude-haiku",
		"gemini", "qwen2-vl", "qwen-vl", "qwen2.5-vl", "qwen3-vl",
	}
	for _, p := range positive {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func (p *VisionPolicy) probeModelsAPI(ctx context.Context, baseURL, apiKey, modelID string) (supports bool, ok bool) {
	client := p.http
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return false, false
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return false, false
	}
	var payload struct {
		Data []struct {
			ID           string `json:"id"`
			Architecture *struct {
				Modality         string   `json:"modality"`
				InputModalities  []string `json:"input_modalities"`
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || len(payload.Data) == 0 {
		return false, false
	}
	want := strings.TrimSpace(modelID)
	for _, m := range payload.Data {
		if m.ID != want {
			continue
		}
		if m.Architecture == nil {
			return HeuristicModelSupportsImage(want), true
		}
		for _, mod := range m.Architecture.InputModalities {
			if strings.EqualFold(mod, "image") {
				return true, true
			}
		}
		if strings.Contains(strings.ToLower(m.Architecture.Modality), "image") {
			return true, true
		}
		return false, true
	}
	// Model not in catalog — fall back to heuristic but treat as probed=false
	return false, false
}
