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

// Effort modes / levels for BP_LLM_EFFORT.
const (
	EffortAuto    = "auto"
	EffortNone    = "none"
	EffortMinimal = "minimal"
	EffortLow     = "low"
	EffortMedium  = "medium"
	EffortHigh    = "high"
	EffortXHigh   = "xhigh"
	EffortMax     = "max"
)

// effortRank is ascending compute budget (for nearest-level mapping).
var effortRank = map[string]int{
	EffortNone:    0,
	EffortMinimal: 1,
	EffortLow:     2,
	EffortMedium:  3,
	EffortHigh:    4,
	EffortXHigh:   5,
	EffortMax:     6,
}

// modelEffortInfo is cached catalog capability for one baseURL+model.
type modelEffortInfo struct {
	Supports          bool
	SupportedEfforts  []string // empty + Supports → any gateway effort ok
	DefaultEffort     string
	Mandatory         bool
	SupportsMaxTokens bool
}

// EffortPolicy decides whether / which reasoning effort to attach to LLM requests.
type EffortPolicy struct {
	mode      string
	providers map[string]config.LLMProvider
	active    string
	http      *http.Client

	mu    sync.Mutex
	cache map[string]modelEffortInfo // key: baseURL + "\x00" + modelID
}

// NewEffortPolicy builds a gate from app LLM config.
// Empty cfg.Effort disables effort injection (no catalog probe) — used by unit tests
// that construct Client without FromApp. Production always sets Effort via FromApp.
func NewEffortPolicy(cfg Config) *EffortPolicy {
	raw := strings.TrimSpace(cfg.Effort)
	mode := ""
	if raw != "" {
		mode = config.ParseEffortMode(raw)
	}
	return &EffortPolicy{
		mode:      mode,
		providers: cfg.Providers,
		active:    cfg.ActiveProvider,
		http:      &http.Client{Timeout: 12 * time.Second},
		cache:     map[string]modelEffortInfo{},
	}
}

// Mode returns the normalized effort config mode (auto when unset/nil).
func (p *EffortPolicy) Mode() string {
	if p == nil || p.mode == "" {
		return EffortAuto
	}
	return p.mode
}

// ResolveFor returns the effort level to send for this provider, or "" to omit the param.
func (p *EffortPolicy) ResolveFor(ctx context.Context, prov config.LLMProvider) string {
	if p == nil || p.mode == "" {
		return ""
	}
	return p.ResolveWithMode(ctx, prov, p.mode)
}

// ResolveWithMode resolves effort for an explicit mode (per-turn override or config).
// Empty mode → omit. Mode "auto" follows catalog defaults like BP_LLM_EFFORT=auto.
func (p *EffortPolicy) ResolveWithMode(ctx context.Context, prov config.LLMProvider, mode string) string {
	if p == nil {
		return ""
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return ""
	}
	mode = config.ParseEffortMode(mode)
	info := p.lookup(ctx, prov)
	wanted := wantedLevelFor(mode, info)
	if wanted == "" {
		return ""
	}
	return mapToSupported(wanted, info)
}

func (p *EffortPolicy) wantedLevel(info modelEffortInfo) string {
	return wantedLevelFor(p.mode, info)
}

func wantedLevelFor(mode string, info modelEffortInfo) string {
	switch mode {
	case EffortAuto:
		if !info.Supports {
			return ""
		}
		def := strings.TrimSpace(info.DefaultEffort)
		if def == "" {
			return EffortMedium
		}
		if def == EffortNone {
			// Catalog "none" means reasoning off by default — omit under auto.
			return ""
		}
		return config.ParseEffortMode(def)
	default:
		// Explicit level: only when model supports effort (avoid 400s).
		if !info.Supports {
			return ""
		}
		if mode == EffortNone && info.Mandatory {
			return "" // mandatory models reject effort:none
		}
		return mode
	}
}

func (p *EffortPolicy) lookup(ctx context.Context, prov config.LLMProvider) modelEffortInfo {
	modelID := strings.TrimSpace(prov.Model)
	baseURL := strings.TrimRight(strings.TrimSpace(prov.BaseURL), "/")
	apiKey := strings.TrimSpace(prov.APIKey)
	if modelID == "" {
		return modelEffortInfo{}
	}
	cacheKey := baseURL + "\x00" + modelID
	p.mu.Lock()
	if v, ok := p.cache[cacheKey]; ok {
		p.mu.Unlock()
		return v
	}
	p.mu.Unlock()

	info := modelEffortInfo{}
	probed := false
	if baseURL != "" {
		if hit, ok := p.probeModelsAPI(ctx, baseURL, apiKey, modelID); ok {
			info = hit
			probed = true
		}
	}
	if !probed {
		if HeuristicModelSupportsEffort(modelID) {
			info = modelEffortInfo{Supports: true, DefaultEffort: EffortMedium}
		}
	}

	p.mu.Lock()
	p.cache[cacheKey] = info
	p.mu.Unlock()
	return info
}

func mapToSupported(wanted string, info modelEffortInfo) string {
	wanted = config.ParseEffortMode(wanted)
	if wanted == EffortAuto {
		wanted = EffortMedium
	}
	if len(info.SupportedEfforts) == 0 {
		return wanted
	}
	allowed := make([]string, 0, len(info.SupportedEfforts))
	for _, e := range info.SupportedEfforts {
		n := config.ParseEffortMode(e)
		if n == EffortAuto {
			continue
		}
		if _, ok := effortRank[n]; ok {
			allowed = append(allowed, n)
		}
	}
	if len(allowed) == 0 {
		return wanted
	}
	for _, e := range allowed {
		if e == wanted {
			return wanted
		}
	}
	// Nearest by rank.
	wantRank, ok := effortRank[wanted]
	if !ok {
		return allowed[0]
	}
	best := allowed[0]
	bestDist := absInt(effortRank[best] - wantRank)
	for _, e := range allowed[1:] {
		d := absInt(effortRank[e] - wantRank)
		if d < bestDist {
			best, bestDist = e, d
		}
	}
	if best == EffortNone && info.Mandatory {
		for _, e := range allowed {
			if e != EffortNone {
				return e
			}
		}
	}
	return best
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// HeuristicModelSupportsEffort is a best-effort local check when Models API is unavailable.
// Conservative: only known reasoning / thinking families — omit for plain chat models.
func HeuristicModelSupportsEffort(modelID string) bool {
	s := strings.ToLower(strings.TrimSpace(modelID))
	if s == "" {
		return false
	}
	positive := []string{
		"o1", "o3", "o4-mini", "o4-",
		"gpt-5", "gpt5",
		"reasoning", "thinking",
		"deepseek-r1", "deepseek-reasoner",
		"claude-3-7", "claude-4", "claude-opus-4", "claude-sonnet-4",
		"gemini-2.5", "gemini-3",
		"grok-3", "grok-4",
		"qwq", "qwen3", // many Qwen3 variants expose thinking
	}
	for _, p := range positive {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// ApplyEffort mutates the request body for chat or responses when effort is non-empty.
// Chat: OpenAI top-level reasoning_effort + OpenRouter unified reasoning.effort.
// Responses: nested reasoning.effort only (OpenAI Responses / OpenRouter).
func ApplyEffort(body map[string]any, api string, effort string) {
	effort = strings.TrimSpace(effort)
	if body == nil || effort == "" {
		return
	}
	effort = config.ParseEffortMode(effort)
	if effort == EffortAuto {
		return
	}
	api = strings.ToLower(strings.TrimSpace(api))
	reasoning := map[string]any{"effort": effort}
	if api == "responses" {
		body["reasoning"] = reasoning
		return
	}
	// chat completions
	body["reasoning"] = reasoning
	body["reasoning_effort"] = effort
}

func (p *EffortPolicy) probeModelsAPI(ctx context.Context, baseURL, apiKey, modelID string) (modelEffortInfo, bool) {
	client := p.http
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return modelEffortInfo{}, false
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return modelEffortInfo{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return modelEffortInfo{}, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return modelEffortInfo{}, false
	}
	var payload struct {
		Data []struct {
			ID                  string   `json:"id"`
			SupportedParameters []string `json:"supported_parameters"`
			Reasoning           *struct {
				SupportedEfforts  []string `json:"supported_efforts"`
				DefaultEffort     string   `json:"default_effort"`
				Mandatory         bool     `json:"mandatory"`
				SupportsMaxTokens bool     `json:"supports_max_tokens"`
			} `json:"reasoning"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || len(payload.Data) == 0 {
		return modelEffortInfo{}, false
	}
	want := strings.TrimSpace(modelID)
	for _, m := range payload.Data {
		if m.ID != want {
			continue
		}
		info := modelEffortInfo{}
		if m.Reasoning != nil {
			// Presence of reasoning object = effort selection advertised (OpenRouter).
			info.Supports = true
			info.DefaultEffort = strings.TrimSpace(m.Reasoning.DefaultEffort)
			info.Mandatory = m.Reasoning.Mandatory
			info.SupportsMaxTokens = m.Reasoning.SupportsMaxTokens
			if m.Reasoning.SupportedEfforts != nil {
				info.SupportedEfforts = append([]string{}, m.Reasoning.SupportedEfforts...)
			}
			return info, true
		}
		for _, sp := range m.SupportedParameters {
			sp = strings.ToLower(strings.TrimSpace(sp))
			if sp == "reasoning" || sp == "reasoning_effort" || sp == "include_reasoning" {
				info.Supports = true
				info.DefaultEffort = EffortMedium
				return info, true
			}
		}
		// Model found but no reasoning advertisement → unsupported (probed).
		return modelEffortInfo{Supports: false}, true
	}
	// Model not in catalog — not a successful probe.
	return modelEffortInfo{}, false
}
