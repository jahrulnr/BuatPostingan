package entity

// ProviderDefinition describes a supported provider family. It contains no
// credentials and is safe to return through the settings API.
type ProviderDefinition struct {
	Type           string `json:"type"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	AuthType       string `json:"auth_type"` // api_key | local | oauth
	API            string `json:"api"`       // chat | responses | messages
	BaseURL        string `json:"base_url,omitempty"`
	Prefix         string `json:"prefix,omitempty"`
	Icon           string `json:"icon"`
	Accent         string `json:"accent"`
	Configurable   bool   `json:"configurable"`
	APIKeyOptional bool   `json:"api_key_optional,omitempty"`
	Note           string `json:"note,omitempty"`
}
