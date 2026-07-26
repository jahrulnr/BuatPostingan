package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is runtime config. Process-level knobs come from BP_* env; product
// knobs are hardcoded defaults overridden by storage/config.json via
// ApplySettingsFile. LLM providers come solely from config.json.
type Config struct {
	HTTPAddr    string
	WebRoot     string
	StorageRoot string
	DocsRoot    string
	PromptsRoot string
	ToolsRoot   string
	SkillsRoot  string
	// WorkspaceRoot is the agent's logical working directory (cwd surfaced to
	// the LLM via developer.md). Empty → process cwd at Load() time.
	WorkspaceRoot string

	MaxToolRounds       int
	SpeakFloorTTL       int
	LockTTL             int
	TurnRateLimitPerMin int
	TurnJobTimeoutSec   int

	LLMStub   bool
	LLMStream bool // request stream=true (default); false → JSON non-stream
	// LLMVision: auto|on|off — gate multimodal image parts (default auto).
	LLMVision string
	// LLMEffort: auto|none|minimal|low|medium|high|xhigh|max — reasoning effort (default auto).
	LLMEffort                  string
	LLMStrategy                string
	LLMActiveProvider          string
	LLMTotalAttemptBudget      int
	LLMCircuitFailureThreshold int
	LLMCircuitCooldownSec      int
	LLMRetryStatuses           []int
	// Retry backoff between transient attempts (exp + bounded jitter, capped).
	LLMRetryBaseDelayMS int
	LLMRetryMaxDelayMS  int
	LLMRetryJitter      float64
	LLMProviders        map[string]LLMProvider
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

	// GitHubToken (env: BP_GITHUB_TOKEN / GITHUB_TOKEN) — optional web_search rate limit.
	GitHubToken string

	// MCP (Model Context Protocol) — servers usually come from config.json.
	MCPEnabled           bool
	MCPConnectTimeoutSec int
	MCPCallTimeoutSec    int
	MCPServers           []MCPServer
}

// MCPServer is a runtime MCP server slot (stdio MVP).
type MCPServer struct {
	ID             string
	Transport      string // stdio | sse | http
	Command        string
	Args           []string
	Env            map[string]string
	URL            string
	Enabled        bool
	Trusted        bool
	AllowTools     []string
	DenyTools      []string
	AllowMutations bool
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

	// No env-based providers — config.json is the sole source for LLM providers.
	// Stub defaults to true (ready-to-use); set BP_LLM_STUB=false after configuring
	// providers in storage/config.json.
	stub := getenvBoolFirst([]string{"BP_LLM_STUB"}, true)

	cfg := Config{
		// Process-level paths (env-only — never in config.json).
		HTTPAddr:    getenvFirst("BP_HTTP_ADDR", ":8080"),
		WebRoot:     getenvFirst("BP_WEB_ROOT", "web"),
		StorageRoot: getenvFirst("BP_STORAGE_ROOT", "storage/webchat"),
		DocsRoot:    getenvFirst("BP_DOCS_ROOT", "resources/webchat/docs"),
		PromptsRoot: getenvFirst("BP_PROMPTS_ROOT", "resources/webchat/prompts"),
		ToolsRoot:   getenvFirst("BP_TOOLS_ROOT", "resources/webchat/tools"),
		SkillsRoot:  getenvFirst("BP_SKILLS_ROOT", "resources/webchat/skills"),
		// Default to absolute process cwd so the LLM sees a stable path even
		// if the binary is launched from elsewhere; explicit env wins.
		WorkspaceRoot: resolveWorkspace(getenvFirst("BP_WORKSPACE_ROOT", ".")),

		// Env-only toggles (not in config.json).
		LLMStub:          stub,
		LLMRetryStatuses: parseIntList(getenvFirst("BP_LLM_RETRY_STATUSES", "408,409,413,425,429,500,502,503,504")),

		// Product knobs — hardcoded defaults; config.json overrides via ApplySettingsFile.
		MaxToolRounds:              8,
		SpeakFloorTTL:              600,
		LockTTL:                    300,
		TurnRateLimitPerMin:        10,
		TurnJobTimeoutSec:          120,
		LLMStream:                  true,
		LLMVision:                  "auto",
		LLMEffort:                  "auto",
		LLMStrategy:                "failover",
		LLMActiveProvider:          "",
		LLMTotalAttemptBudget:      4,
		LLMCircuitFailureThreshold: 3,
		LLMCircuitCooldownSec:      60,
		LLMRetryBaseDelayMS:        250,
		LLMRetryMaxDelayMS:         5000,
		LLMRetryJitter:             0.2,
		LLMProviders:               nil,
		ContextCompactionEnabled:   true,
		ContextMaxInputTokens:      12000,
		ContextReserveTokens:       3000,
		ContextRecentTurns:         4,
		ContextSummaryMaxChars:     12000,
		DocsTopK:                   5,
		DocsMinScore:               0.5,
		DocsFuzzyEnabled:           true,
		DocsAppID:                  "buatpostingan",
		GitHubToken:                "",
		MCPEnabled:                 true,
		MCPConnectTimeoutSec:       15,
		MCPCallTimeoutSec:          30,
		MCPServers:                 nil,
	}

	switch cfg.LLMStrategy {
	case "failover", "round_robin", "switch":
	default:
		cfg.LLMStrategy = "failover"
	}
	return cfg
}

// ParseVisionMode normalizes vision mode to auto|on|off (default auto).
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

// resolveWorkspace turns the env value into an absolute path. Empty or "."
// → process cwd. Errors fall back to the raw input (never crash boot).
func resolveWorkspace(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" || v == "." {
		if cwd, err := os.Getwd(); err == nil {
			return cwd
		}
		return v
	}
	if abs, err := filepath.Abs(v); err == nil {
		return abs
	}
	return v
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
