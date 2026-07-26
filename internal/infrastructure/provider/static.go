package provider

import (
	"fmt"
	"strings"

	"buatpostingan/internal/domain/entity"
)

type staticAdapter struct {
	def   entity.ProviderDefinition
	match func(entity.SettingsProvider) bool
}

// NewStatic builds an adapter for providers that share the standard
// connection fields. Provider-specific wire behavior remains selected by API.
func NewStatic(def entity.ProviderDefinition, match func(entity.SettingsProvider) bool) Adapter {
	return &staticAdapter{def: def, match: match}
}

func (a *staticAdapter) Definition() entity.ProviderDefinition { return a.def }

func (a *staticAdapter) Matches(p entity.SettingsProvider) bool {
	return a.match != nil && a.match(p)
}

func (a *staticAdapter) Normalize(p entity.SettingsProvider) (entity.SettingsProvider, error) {
	p = ApplyDefaults(p, a.def)
	p.ID = strings.ToUpper(strings.TrimSpace(p.ID))
	p.Name = strings.TrimSpace(p.Name)
	p.Prefix = strings.TrimSpace(p.Prefix)
	p.API = strings.ToLower(strings.TrimSpace(p.API))
	p.BaseURL = strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if p.ID == "" {
		return entity.SettingsProvider{}, fmt.Errorf("provider id required")
	}
	if p.BaseURL == "" {
		return entity.SettingsProvider{}, fmt.Errorf("provider base_url required")
	}
	switch p.API {
	case "chat", "responses", "messages":
	default:
		return entity.SettingsProvider{}, fmt.Errorf("provider api must be chat|responses|messages")
	}
	return p, nil
}
