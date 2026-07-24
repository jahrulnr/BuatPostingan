package entity

// SettingsFile is the on-disk JSON document (storage/config.json).
type SettingsFile struct {
	Version int           `json:"version"`
	Users   []SettingsUser `json:"users"`
	LLM     SettingsLLM   `json:"llm"`
}

// SettingsUser is a minimal local user row (no auth in v1).
type SettingsUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // owner | admin | member
}

// SettingsLLM holds strategy + provider slots.
type SettingsLLM struct {
	Strategy       string             `json:"strategy,omitempty"`
	ActiveProvider string             `json:"active_provider,omitempty"`
	Stub           *bool              `json:"stub"`
	Providers      []SettingsProvider `json:"providers"`
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
