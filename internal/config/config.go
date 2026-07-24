package config

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Config is env-based runtime config (BP_* product env only).
type Config struct {
	HTTPAddr    string
	WebRoot     string
	StorageRoot string
	DocsRoot    string
	PromptsRoot string
	ToolsRoot   string

	WriteEnabled        bool
	MaxToolRounds       int
	SpeakFloorTTL       int
	LockTTL             int
	TurnRateLimitPerMin int
	TurnJobTimeoutSec   int

	LLMStub                    bool
	LLMStream                  bool // request stream=true (default); false → JSON non-stream
	// LLMVision: auto|on|off — gate multimodal image parts (default auto).
	LLMVision                  string
	// LLMEffort: auto|none|minimal|low|medium|high|xhigh|max — reasoning effort (default auto).
	LLMEffort                  string
	LLMStrategy                string
	LLMActiveProvider          string
	LLMTotalAttemptBudget      int
	LLMCircuitFailureThreshold int
	LLMCircuitCooldownSec      int
	LLMRetryStatuses           []int
	LLMProviders               map[string]LLMProvider
	// LLMModelLists is optional provider→model ids from config.json (picker expand).
	LLMModelLists map[string][]string

	ContextCompactionEnabled bool
	ContextMaxInputTokens    int
	ContextReserveTokens     int
	ContextRecentTurns       int
	ContextSummaryMaxChars   int

	DocsTopK         int
	DocsMinScore     float64
	DocsFuzzyEnabled bool
	DocsAppID        string
}

// LLMProvider is one OpenAI-compatible upstream.
type LLMProvider struct {
	ID              string
	BaseURL         string
	APIKey          string
	Model           string
	API             string // chat | responses
	TimeoutSec      int
	MaxAttempts     int
	Weight          int
	ContextWindow   int
	MaxOutputTokens int
	MaxInputTokens  int
	Enabled         bool
}

func Load() Config {
	_ = loadDotEnv(".env")

	providers := loadProviders()
	anyKey := false
	for _, p := range providers {
		if strings.TrimSpace(p.APIKey) != "" {
			anyKey = true
			break
		}
	}

	stub := getenvBoolFirst([]string{"BP_LLM_STUB"}, !anyKey)

	active := strings.ToUpper(strings.TrimSpace(getenvFirst("BP_LLM_ACTIVE_PROVIDER", "")))
	if active == "" {
		ids := make([]string, 0, len(providers))
		for id := range providers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if providers[id].Enabled {
				active = id
				break
			}
		}
	}

	cfg := Config{
		HTTPAddr:    getenvFirst("BP_HTTP_ADDR", ":8080"),
		WebRoot:     getenvFirst("BP_WEB_ROOT", "web"),
		StorageRoot: getenvFirst("BP_STORAGE_ROOT", "storage/webchat"),
		DocsRoot:    getenvFirst("BP_DOCS_ROOT", "docs/webchat"),
		PromptsRoot: getenvFirst("BP_PROMPTS_ROOT", "resources/webchat/prompts"),
		ToolsRoot:   getenvFirst("BP_TOOLS_ROOT", "resources/webchat/tools"),

		WriteEnabled:        false, // hard false — reader/instructor only
		MaxToolRounds:       getenvIntFirst([]string{"BP_MAX_TOOL_ROUNDS"}, 8),
		SpeakFloorTTL:       getenvIntFirst([]string{"BP_SPEAK_FLOOR_TTL_SEC"}, 600),
		LockTTL:             getenvIntFirst([]string{"BP_LOCK_TTL_SEC"}, 300),
		TurnRateLimitPerMin: getenvIntFirst([]string{"BP_TURN_RATE_LIMIT_PER_MIN"}, 10),
		TurnJobTimeoutSec:   getenvIntFirst([]string{"BP_TURN_JOB_TIMEOUT_SEC"}, 120),

		LLMStub:                    stub,
		LLMStream:                  getenvBoolFirst([]string{"BP_LLM_STREAM"}, true),
		LLMVision:                  ParseVisionMode(getenvFirst("BP_LLM_VISION", "auto")),
		LLMEffort:                  ParseEffortMode(getenvFirst("BP_LLM_EFFORT", "auto")),
		LLMStrategy:                strings.ToLower(getenvFirst("BP_LLM_STRATEGY", "failover")),
		LLMActiveProvider:          active,
		LLMTotalAttemptBudget:      getenvIntFirst([]string{"BP_LLM_TOTAL_ATTEMPT_BUDGET"}, 4),
		LLMCircuitFailureThreshold: getenvIntFirst([]string{"BP_LLM_CIRCUIT_FAILURE_THRESHOLD"}, 3),
		LLMCircuitCooldownSec:      getenvIntFirst([]string{"BP_LLM_CIRCUIT_COOLDOWN_SEC"}, 60),
		LLMRetryStatuses:           parseIntList(getenvFirst("BP_LLM_RETRY_STATUSES", "408,409,413,425,429,500,502,503,504")),
		LLMProviders:               providers,

		ContextCompactionEnabled: getenvBoolFirst([]string{"BP_CONTEXT_COMPACTION_ENABLED"}, false),
		ContextMaxInputTokens:    getenvIntFirst([]string{"BP_CONTEXT_MAX_INPUT_TOKENS"}, 12000),
		ContextReserveTokens:     getenvIntFirst([]string{"BP_CONTEXT_RESERVE_TOKENS"}, 3000),
		ContextRecentTurns:       getenvIntFirst([]string{"BP_CONTEXT_RECENT_TURNS"}, 4),
		ContextSummaryMaxChars:   getenvIntFirst([]string{"BP_CONTEXT_SUMMARY_MAX_CHARS"}, 12000),

		DocsTopK:         getenvIntFirst([]string{"BP_DOCS_TOP_K"}, 5),
		DocsMinScore:     getenvFloatFirst([]string{"BP_DOCS_MIN_SCORE"}, 0.5),
		DocsFuzzyEnabled: getenvBoolFirst([]string{"BP_DOCS_FUZZY_ENABLED"}, true),
		DocsAppID:        getenvFirst("BP_DOCS_APP_ID", "buatpostingan"),
	}

	switch cfg.LLMStrategy {
	case "failover", "round_robin", "switch":
	default:
		cfg.LLMStrategy = "failover"
	}
	return cfg
}

func loadProviders() map[string]LLMProvider {
	raw := getenvFirst("BP_LLM_PROVIDERS", "OPENROUTER")
	ids := strings.Split(raw, ",")
	out := make(map[string]LLMProvider, len(ids))
	for _, rawID := range ids {
		id := strings.ToUpper(strings.TrimSpace(rawID))
		if id == "" {
			continue
		}
		prefix := "BP_LLM_" + id + "_"
		p := LLMProvider{
			ID:              id,
			BaseURL:         strings.TrimRight(getenvFirst(prefix+"BASE_URL", ""), "/"),
			APIKey:          getenvFirst(prefix+"API_KEY", ""),
			Model:           getenvFirst(prefix+"MODEL", ""),
			API:             strings.ToLower(getenvFirst(prefix+"API", "responses")),
			TimeoutSec:      getenvIntFirst([]string{prefix + "TIMEOUT_SEC", "BP_LLM_TIMEOUT_SEC"}, 60),
			MaxAttempts:     getenvIntFirst([]string{prefix + "MAX_ATTEMPTS"}, 1),
			Weight:          getenvIntFirst([]string{prefix + "WEIGHT"}, 1),
			ContextWindow:   getenvIntFirst([]string{prefix + "CONTEXT_WINDOW", "BP_LLM_CONTEXT_WINDOW"}, 131072),
			MaxOutputTokens: getenvIntFirst([]string{prefix + "MAX_OUTPUT_TOKENS", "BP_LLM_MAX_OUTPUT_TOKENS"}, 4096),
			MaxInputTokens:  getenvIntFirst([]string{prefix + "MAX_INPUT_TOKENS", "BP_CONTEXT_MAX_INPUT_TOKENS"}, 12000),
			Enabled:         getenvBoolFirst([]string{prefix + "ENABLED"}, true),
		}
		if p.API != "chat" && p.API != "responses" {
			p.API = "responses"
		}
		if p.MaxAttempts < 1 {
			p.MaxAttempts = 1
		}
		budget := p.ContextWindow - p.MaxOutputTokens - 512
		if budget < 1000 {
			budget = 1000
		}
		if p.MaxInputTokens > budget {
			p.MaxInputTokens = budget
		}
		out[id] = p
	}
	return out
}

// ParseVisionMode normalizes BP_LLM_VISION to auto|on|off (default auto).
func ParseVisionMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "on", "1", "true", "yes":
		return "on"
	case "off", "0", "false", "no":
		return "off"
	case "auto":
		return "auto"
	default:
		return "auto"
	}
}

// ParseEffortMode normalizes BP_LLM_EFFORT (default auto).
// Accepted: auto|none|minimal|low|medium|high|xhigh|max (+ aliases).
func ParseEffortMode(raw string) string {
	if n, ok := NormalizeEffortOverride(raw); ok && n != "" {
		return n
	}
	if strings.TrimSpace(raw) == "" {
		return "auto"
	}
	// Unknown env values fall back to auto (same as historical Load behavior).
	return "auto"
}

// EffortPickerOptions is the global reasoning-effort menu (includes auto).
func EffortPickerOptions() []string {
	return []string{"auto", "none", "minimal", "low", "medium", "high", "xhigh", "max"}
}

// NormalizeEffortOverride parses a StartTurn / picker effort override.
// Empty input → ("", true) meaning “use server default”.
// Unknown non-empty → ("", false).
func NormalizeEffortOverride(raw string) (normalized string, ok bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", true
	}
	switch s {
	case "auto":
		return "auto", true
	case "none", "off", "0", "false", "no":
		return "none", true
	case "minimal", "min":
		return "minimal", true
	case "low":
		return "low", true
	case "medium", "med", "default":
		return "medium", true
	case "high":
		return "high", true
	case "xhigh", "x-high", "extra", "extra_high", "extrahigh":
		return "xhigh", true
	case "max", "maximum":
		return "max", true
	default:
		return "", false
	}
}

func getenvFirst(keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	fallback := keys[len(keys)-1]
	for _, k := range keys[:len(keys)-1] {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return fallback
}

func getenvIntFirst(keys []string, fallback int) int {
	for _, k := range keys {
		v := strings.TrimSpace(os.Getenv(k))
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		return n
	}
	return fallback
}

func getenvFloatFirst(keys []string, fallback float64) float64 {
	for _, k := range keys {
		v := strings.TrimSpace(os.Getenv(k))
		if v == "" {
			continue
		}
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			continue
		}
		return n
	}
	return fallback
}

func getenvBoolFirst(keys []string, fallback bool) bool {
	for _, k := range keys {
		v := strings.TrimSpace(os.Getenv(k))
		if v == "" {
			continue
		}
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func parseIntList(raw string) []int {
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n <= 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

// loadDotEnv sets KEY=VAL from a simple .env file when the key is not already set.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}
