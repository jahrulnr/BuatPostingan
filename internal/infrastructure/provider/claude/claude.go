package claude

import (
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/infrastructure/provider"
)

func New() provider.Adapter {
	def := entity.ProviderDefinition{
		Type: "claude", Name: "Claude API",
		Description: "Official Anthropic Messages API for Claude models.",
		AuthType:    "api_key", API: "messages", BaseURL: "https://api.anthropic.com/v1",
		Prefix: "claude", Icon: "AI", Accent: "#d97757", Configurable: true,
	}
	return provider.NewStatic(def, func(p entity.SettingsProvider) bool {
		return provider.MatchIDOrHost(p, []string{"claude", "anthropic"}, []string{"api.anthropic.com"})
	})
}
