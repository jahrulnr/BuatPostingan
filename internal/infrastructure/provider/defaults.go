package provider

import (
	"strings"

	"buatpostingan/internal/domain/entity"
)

// ApplyDefaults fills empty connection fields without overwriting explicit
// operator values.
func ApplyDefaults(p entity.SettingsProvider, d entity.ProviderDefinition) entity.SettingsProvider {
	p.Type = d.Type
	if strings.TrimSpace(p.ID) == "" {
		p.ID = strings.ToUpper(strings.ReplaceAll(d.Type, "-", "_"))
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = d.Name
	}
	if strings.TrimSpace(p.Prefix) == "" {
		p.Prefix = d.Prefix
	}
	if strings.TrimSpace(p.API) == "" {
		p.API = d.API
	}
	if strings.TrimSpace(p.BaseURL) == "" {
		p.BaseURL = d.BaseURL
	}
	p.APIKeyOptional = d.APIKeyOptional
	return p
}

func MatchIDOrHost(p entity.SettingsProvider, ids []string, hosts []string) bool {
	id := strings.ToLower(strings.TrimSpace(p.ID))
	base := strings.ToLower(strings.TrimSpace(p.BaseURL))
	for _, candidate := range ids {
		if id == strings.ToLower(candidate) {
			return true
		}
	}
	for _, host := range hosts {
		if strings.Contains(base, strings.ToLower(host)) {
			return true
		}
	}
	return false
}
