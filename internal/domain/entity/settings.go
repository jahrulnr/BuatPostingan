package entity

// SettingsFile is the on-disk JSON document (storage/config.json).
//
// JSON is the source of truth for product knobs (limits, llm globals, context,
// docs, web_search, mcp). Env (BP_*) provides process-level paths and a few
// env-only toggles; product knobs use hardcoded defaults when JSON omits a key.
//
// Env-only (never in JSON): BP_HTTP_ADDR, BP_WEB_ROOT, BP_STORAGE_ROOT,
// BP_DOCS_ROOT, BP_PROMPTS_ROOT, BP_TOOLS_ROOT, BP_SKILLS_ROOT, BP_CONFIG_PATH,
// BP_LLM_STUB, BP_LLM_RETRY_STATUSES
type SettingsFile struct {
	Version   int               `json:"version"`
	Users     []SettingsUser    `json:"users"`
	LLM       SettingsLLM       `json:"llm"`
	Limits    SettingsLimits    `json:"limits,omitempty"`
	Context   SettingsContext   `json:"context,omitempty"`
	Docs      SettingsDocs      `json:"docs,omitempty"`
	WebSearch SettingsWebSearch `json:"web_search,omitempty"`
	MCP       SettingsMCP       `json:"mcp,omitempty"`
}

// SettingsLimits holds turn-loop / concurrency limits (JSON: limits.*).
// Hardcoded defaults in config.Load() apply when JSON omits a key.
type SettingsLimits struct {
	MaxToolRounds     *int `json:"max_tool_rounds,omitempty"`
	SpeakFloorTTLSec  *int `json:"speak_floor_ttl_sec,omitempty"`
	LockTTLSec        *int `json:"lock_ttl_sec,omitempty"`
	TurnJobTimeoutSec *int `json:"turn_job_timeout_sec,omitempty"`
}

// SettingsContext holds context-compaction knobs (JSON: context.*).
// Hardcoded defaults in config.Load() apply when JSON omits a key.
type SettingsContext struct {
	CompactionEnabled *bool `json:"compaction_enabled,omitempty"`
	MaxInputTokens    *int  `json:"max_input_tokens,omitempty"`
	ReserveTokens     *int  `json:"reserve_tokens,omitempty"`
	RecentTurns       *int  `json:"recent_turns,omitempty"`
	SummaryMaxChars   *int  `json:"summary_max_chars,omitempty"`
}

// SettingsDocs holds retrieval knobs (JSON: docs.*).
// Hardcoded defaults in config.Load() apply when JSON omits a key.
type SettingsDocs struct {
	TopK         *int     `json:"top_k,omitempty"`
	MinScore     *float64 `json:"min_score,omitempty"`
	FuzzyEnabled *bool    `json:"fuzzy_enabled,omitempty"`
	AppID        string   `json:"app_id,omitempty"`
}

// SettingsWebSearch holds optional web_search knobs (JSON: web_search.github_token).
type SettingsWebSearch struct {
	GitHubToken string `json:"github_token,omitempty"`
}

// SettingsMCP configures MCP client servers for the webchat agent.
type SettingsMCP struct {
	Enabled           *bool               `json:"enabled,omitempty"`
	ConnectTimeoutSec int                 `json:"connect_timeout_sec,omitempty"`
	CallTimeoutSec    int                 `json:"call_timeout_sec,omitempty"`
	Servers           []SettingsMCPServer `json:"servers,omitempty"`
}

// SettingsMCPServer is one MCP server entry (stdio MVP; url reserved for HTTP).
type SettingsMCPServer struct {
	ID             string            `json:"id"`
	Transport      string            `json:"transport"` // stdio | sse | http
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	URL            string            `json:"url,omitempty"`
	Enabled        bool              `json:"enabled"`
	Trusted        bool              `json:"trusted,omitempty"`
	AllowTools     []string          `json:"allow_tools,omitempty"`
	DenyTools      []string          `json:"deny_tools,omitempty"`
	AllowMutations bool              `json:"allow_mutations,omitempty"`
}

// SettingsUser is a minimal local user row (no auth in v1).
type SettingsUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // owner | admin | member
}

// SettingsLLM holds strategy + provider slots, plus global LLM knobs (JSON:
// llm.stream, llm.vision, llm.effort, llm.total_attempt_budget,
// llm.retry_*). Hardcoded defaults in config.Load() apply when JSON omits a key.
//
// Stub is NOT here — it stays env-only (BP_LLM_STUB) for development.
type SettingsLLM struct {
	Strategy       string             `json:"strategy,omitempty"`
	ActiveProvider string             `json:"active_provider,omitempty"`
	Providers      []SettingsProvider `json:"providers"`

	// Global knobs (omitempty → omit keeps hardcoded default).
	Stream             *bool    `json:"stream,omitempty"`
	Vision             string   `json:"vision,omitempty"` // auto|on|off
	Effort             string   `json:"effort,omitempty"` // auto|none|minimal|low|medium|high|xhigh|max
	TotalAttemptBudget *int     `json:"total_attempt_budget,omitempty"`
	RetryBaseDelayMS   *int     `json:"retry_base_delay_ms,omitempty"`
	RetryMaxDelayMS    *int     `json:"retry_max_delay_ms,omitempty"`
	RetryJitter        *float64 `json:"retry_jitter,omitempty"`
}

// SettingsProvider is one configured LLM upstream (file form).
type SettingsProvider struct {
	Type           string          `json:"type,omitempty"`
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Prefix         string          `json:"prefix,omitempty"`
	API            string          `json:"api"` // chat | responses | messages
	BaseURL        string          `json:"base_url"`
	APIKey         string          `json:"api_key,omitempty"`
	APIKeys        []string        `json:"api_keys,omitempty"` // reserved; unused in v1
	APIKeyOptional bool            `json:"api_key_optional,omitempty"`
	Enabled        bool            `json:"enabled"`
	Models         []SettingsModel `json:"models"`
	TimeoutSec     int             `json:"timeout_sec,omitempty"`
	MaxAttempts    int             `json:"max_attempts,omitempty"`
	Weight         int             `json:"weight,omitempty"`
}

// SettingsModel is a selectable model id under a provider.
type SettingsModel struct {
	ID            string   `json:"id"`
	Label         string   `json:"label,omitempty"`
	Task          string   `json:"task,omitempty"`           // chat, embedding, speech-to-text, image-generation, ...
	ContextWindow int      `json:"context_window,omitempty"` // max input tokens
	MaxOutput     int      `json:"max_output,omitempty"`     // max output tokens
	InputModes    []string `json:"input_modes,omitempty"`    // text, image, file, pdf
	OutputModes   []string `json:"output_modes,omitempty"`   // text, image
	EffortLevels  []string `json:"effort_levels,omitempty"`  // low, medium, high, max, xhigh
	SupportsTools bool     `json:"supports_tools,omitempty"`
	Description   string   `json:"description,omitempty"`
}
