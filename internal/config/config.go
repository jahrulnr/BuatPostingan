package config

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Config is env-based runtime config.
// Prefer WEBCHAT_* when embedding this AI kit in another product.
// BP_* is the BuatPostingan product alias (checked first in this repo).
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
	LLMStrategy                string
	LLMActiveProvider          string
	LLMTotalAttemptBudget      int
	LLMCircuitFailureThreshold int
	LLMCircuitCooldownSec      int
	LLMRetryStatuses           []int
	LLMProviders               map[string]LLMProvider

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

	stub := getenvBoolFirst([]string{"BP_LLM_STUB", "WEBCHAT_LLM_STUB"}, !anyKey)

	active := strings.ToUpper(strings.TrimSpace(getenvFirst("BP_LLM_ACTIVE_PROVIDER", "WEBCHAT_LLM_ACTIVE_PROVIDER", "")))
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
		HTTPAddr:    getenvFirst("BP_HTTP_ADDR", "WEBCHAT_HTTP_ADDR", ":8080"),
		WebRoot:     getenvFirst("BP_WEB_ROOT", "WEBCHAT_WEB_ROOT", "web"),
		StorageRoot: getenvFirst("BP_STORAGE_ROOT", "WEBCHAT_STORAGE_ROOT", "storage/webchat"),
		DocsRoot:    getenvFirst("BP_DOCS_ROOT", "WEBCHAT_DOCS_ROOT", "docs/webchat"),
		PromptsRoot: getenvFirst("BP_PROMPTS_ROOT", "WEBCHAT_PROMPTS_ROOT", "resources/webchat/prompts"),
		ToolsRoot:   getenvFirst("BP_TOOLS_ROOT", "WEBCHAT_TOOLS_ROOT", "resources/webchat/tools"),

		WriteEnabled:        false, // hard false — reader/instructor only
		MaxToolRounds:       getenvIntFirst([]string{"BP_MAX_TOOL_ROUNDS", "WEBCHAT_MAX_TOOL_ROUNDS"}, 8),
		SpeakFloorTTL:       getenvIntFirst([]string{"BP_SPEAK_FLOOR_TTL_SEC", "WEBCHAT_SPEAK_FLOOR_TTL_SEC"}, 600),
		LockTTL:             getenvIntFirst([]string{"BP_LOCK_TTL_SEC", "WEBCHAT_LOCK_TTL_SEC"}, 300),
		TurnRateLimitPerMin: getenvIntFirst([]string{"BP_TURN_RATE_LIMIT_PER_MIN", "WEBCHAT_TURN_RATE_LIMIT_PER_MIN"}, 10),
		TurnJobTimeoutSec:   getenvIntFirst([]string{"BP_TURN_JOB_TIMEOUT_SEC", "WEBCHAT_TURN_JOB_TIMEOUT_SEC"}, 120),

		LLMStub:                    stub,
		LLMStrategy:                strings.ToLower(getenvFirst("BP_LLM_STRATEGY", "WEBCHAT_LLM_STRATEGY", "failover")),
		LLMActiveProvider:          active,
		LLMTotalAttemptBudget:      getenvIntFirst([]string{"BP_LLM_TOTAL_ATTEMPT_BUDGET", "WEBCHAT_LLM_TOTAL_ATTEMPT_BUDGET"}, 4),
		LLMCircuitFailureThreshold: getenvIntFirst([]string{"BP_LLM_CIRCUIT_FAILURE_THRESHOLD", "WEBCHAT_LLM_CIRCUIT_FAILURE_THRESHOLD"}, 3),
		LLMCircuitCooldownSec:      getenvIntFirst([]string{"BP_LLM_CIRCUIT_COOLDOWN_SEC", "WEBCHAT_LLM_CIRCUIT_COOLDOWN_SEC"}, 60),
		LLMRetryStatuses:           parseIntList(getenvFirst("BP_LLM_RETRY_STATUSES", "WEBCHAT_LLM_RETRY_STATUSES", "408,409,413,425,429,500,502,503,504")),
		LLMProviders:               providers,

		ContextCompactionEnabled: getenvBoolFirst([]string{"BP_CONTEXT_COMPACTION_ENABLED", "WEBCHAT_CONTEXT_COMPACTION_ENABLED"}, false),
		ContextMaxInputTokens:    getenvIntFirst([]string{"BP_CONTEXT_MAX_INPUT_TOKENS", "WEBCHAT_CONTEXT_MAX_INPUT_TOKENS"}, 12000),
		ContextReserveTokens:     getenvIntFirst([]string{"BP_CONTEXT_RESERVE_TOKENS", "WEBCHAT_CONTEXT_RESERVE_TOKENS"}, 3000),
		ContextRecentTurns:       getenvIntFirst([]string{"BP_CONTEXT_RECENT_TURNS", "WEBCHAT_CONTEXT_RECENT_TURNS"}, 4),
		ContextSummaryMaxChars:   getenvIntFirst([]string{"BP_CONTEXT_SUMMARY_MAX_CHARS", "WEBCHAT_CONTEXT_SUMMARY_MAX_CHARS"}, 12000),

		DocsTopK:         getenvIntFirst([]string{"BP_DOCS_TOP_K", "WEBCHAT_DOCS_TOP_K"}, 5),
		DocsMinScore:     getenvFloatFirst([]string{"BP_DOCS_MIN_SCORE", "WEBCHAT_DOCS_MIN_SCORE"}, 0.5),
		DocsFuzzyEnabled: getenvBoolFirst([]string{"BP_DOCS_FUZZY_ENABLED", "WEBCHAT_DOCS_FUZZY_ENABLED"}, true),
		DocsAppID:        getenvFirst("BP_DOCS_APP_ID", "WEBCHAT_DOCS_APP_ID", "buatpostingan"),
	}

	switch cfg.LLMStrategy {
	case "failover", "round_robin", "switch":
	default:
		cfg.LLMStrategy = "failover"
	}
	return cfg
}

func loadProviders() map[string]LLMProvider {
	raw := getenvFirst("BP_LLM_PROVIDERS", "WEBCHAT_LLM_PROVIDERS", "OPENROUTER")
	ids := strings.Split(raw, ",")
	out := make(map[string]LLMProvider, len(ids))
	for _, rawID := range ids {
		id := strings.ToUpper(strings.TrimSpace(rawID))
		if id == "" {
			continue
		}
		prefixBP := "BP_LLM_" + id + "_"
		prefixWC := "WEBCHAT_LLM_" + id + "_"
		p := LLMProvider{
			ID:              id,
			BaseURL:         strings.TrimRight(getenvFirst(prefixBP+"BASE_URL", prefixWC+"BASE_URL", ""), "/"),
			APIKey:          getenvFirst(prefixBP+"API_KEY", prefixWC+"API_KEY", ""),
			Model:           getenvFirst(prefixBP+"MODEL", prefixWC+"MODEL", ""),
			API:             strings.ToLower(getenvFirst(prefixBP+"API", prefixWC+"API", "responses")),
			TimeoutSec:      getenvIntFirst([]string{prefixBP + "TIMEOUT_SEC", prefixWC + "TIMEOUT_SEC", "BP_LLM_TIMEOUT_SEC", "WEBCHAT_LLM_TIMEOUT_SEC"}, 60),
			MaxAttempts:     getenvIntFirst([]string{prefixBP + "MAX_ATTEMPTS", prefixWC + "MAX_ATTEMPTS"}, 1),
			Weight:          getenvIntFirst([]string{prefixBP + "WEIGHT", prefixWC + "WEIGHT"}, 1),
			ContextWindow:   getenvIntFirst([]string{prefixBP + "CONTEXT_WINDOW", prefixWC + "CONTEXT_WINDOW", "BP_LLM_CONTEXT_WINDOW", "WEBCHAT_LLM_CONTEXT_WINDOW"}, 131072),
			MaxOutputTokens: getenvIntFirst([]string{prefixBP + "MAX_OUTPUT_TOKENS", prefixWC + "MAX_OUTPUT_TOKENS", "BP_LLM_MAX_OUTPUT_TOKENS", "WEBCHAT_LLM_MAX_OUTPUT_TOKENS"}, 4096),
			MaxInputTokens:  getenvIntFirst([]string{prefixBP + "MAX_INPUT_TOKENS", prefixWC + "MAX_INPUT_TOKENS", "BP_CONTEXT_MAX_INPUT_TOKENS", "WEBCHAT_CONTEXT_MAX_INPUT_TOKENS"}, 12000),
			Enabled:         getenvBoolFirst([]string{prefixBP + "ENABLED", prefixWC + "ENABLED"}, true),
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
