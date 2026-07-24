package entity

// SettingsFile is the on-disk JSON document (storage/config.json).
//
// JSON is the source of truth for product knobs (limits, llm globals, context,
// docs, web_search, skills root, mcp). Env (BP_*) remains the bootstrap: when a
// field is omitted (or the file is missing/old), the env-derived default wins.
type SettingsFile struct {
	Version    int               `json:"version"`
	Users      []SettingsUser    `json:"users"`
	LLM        SettingsLLM       `json:"llm"`
	Limits     SettingsLimits    `json:"limits,omitempty"`
	Context    SettingsContext   `json:"context,omitempty"`
	Docs       SettingsDocs      `json:"docs,omitempty"`
	WebSearch  SettingsWebSearch `json:"web_search,omitempty"`
	SkillsRoot string            `json:"skills_root,omitempty"`
	MCP        SettingsMCP       `json:"mcp,omitempty"`
}

// SettingsLimits holds turn-loop / concurrency limits (env: BP_MAX_TOOL_ROUNDS,
// BP_SPEAK_FLOOR_TTL_SEC, BP_LOCK_TTL_SEC, BP_TURN_RATE_LIMIT_PER_MIN,
// BP_TURN_JOB_TIMEOUT_SEC).
type SettingsLimits struct {
	MaxToolRounds       *int `json:"max_tool_rounds,omitempty"`
	SpeakFloorTTLSec    *int `json:"speak_floor_ttl_sec,omitempty"`
	LockTTLSec          *int `json:"lock_ttl_sec,omitempty"`
	TurnRateLimitPerMin *int `json:"turn_rate_limit_per_min,omitempty"`
	TurnJobTimeoutSec   *int `json:"turn_job_timeout_sec,omitempty"`
}

// SettingsContext holds context-compaction knobs (env: BP_CONTEXT_*).
type SettingsContext struct {
	CompactionEnabled *bool `json:"compaction_enabled,omitempty"`
	MaxInputTokens    *int  `json:"max_input_tokens,omitempty"`
	ReserveTokens     *int  `json:"reserve_tokens,omitempty"`
	RecentTurns       *int  `json:"recent_turns,omitempty"`
	SummaryMaxChars   *int  `json:"summary_max_chars,omitempty"`
}

// SettingsDocs holds retrieval knobs (env: BP_DOCS_TOP_K, BP_DOCS_MIN_SCORE,
// BP_DOCS_FUZZY_ENABLED, BP_DOCS_APP_ID).
type SettingsDocs struct {
	TopK         *int     `json:"top_k,omitempty"`
	MinScore     *float64 `json:"min_score,omitempty"`
	FuzzyEnabled *bool    `json:"fuzzy_enabled,omitempty"`
	AppID        string   `json:"app_id,omitempty"`
}

// SettingsWebSearch holds optional web_search knobs (env: BP_GITHUB_TOKEN).
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

// SettingsLLM holds strategy + provider slots, plus global LLM knobs that used
// to be env-only (BP_LLM_TOTAL_ATTEMPT_BUDGET, BP_LLM_CIRCUIT_*, BP_LLM_RETRY_*,
// BP_LLM_STREAM, BP_LLM_VISION, BP_LLM_EFFORT).
type SettingsLLM struct {
	Strategy       string             `json:"strategy,omitempty"`
	ActiveProvider string             `json:"active_provider,omitempty"`
	Stub           *bool              `json:"stub"`
	Providers      []SettingsProvider `json:"providers"`

	// Global knobs (omitempty → omit keeps env default).
	Stream                  *bool    `json:"stream,omitempty"`
	Vision                  string   `json:"vision,omitempty"` // auto|on|off
	Effort                  string   `json:"effort,omitempty"` // auto|none|minimal|low|medium|high|xhigh|max
	TotalAttemptBudget      *int     `json:"total_attempt_budget,omitempty"`
	CircuitFailureThreshold *int     `json:"circuit_failure_threshold,omitempty"`
	CircuitCooldownSec      *int     `json:"circuit_cooldown_sec,omitempty"`
	RetryBaseDelayMS        *int     `json:"retry_base_delay_ms,omitempty"`
	RetryMaxDelayMS         *int     `json:"retry_max_delay_ms,omitempty"`
	RetryJitter             *float64 `json:"retry_jitter,omitempty"`
}

// SettingsProvider is one OpenAI-compatible upstream (file form).
type SettingsProvider struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Prefix      string          `json:"prefix,omitempty"`
	API         string          `json:"api"` // chat | responses
	BaseURL     string          `json:"base_url"`
	APIKey      string          `json:"api_key,omitempty"`
	APIKeys     []string        `json:"api_keys,omitempty"` // reserved; unused in v1
	Enabled     bool            `json:"enabled"`
	Models      []SettingsModel `json:"models"`
	TimeoutSec  int             `json:"timeout_sec,omitempty"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
	Weight      int             `json:"weight,omitempty"`
}

// SettingsModel is a selectable model id under a provider.
type SettingsModel struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}
