package openrouter

import (
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/infrastructure/provider"
)

func New() provider.Adapter {
	def := entity.ProviderDefinition{
		Type: "openrouter", Name: "OpenRouter",
		Description: "One API for models from many upstream providers.",
		AuthType:    "api_key", API: "chat", BaseURL: "https://openrouter.ai/api/v1",
		Prefix: "openrouter", Icon: "OR", Accent: "#7c5cff", Configurable: true,
	}
	return provider.NewStatic(def, func(p entity.SettingsProvider) bool {
		return provider.MatchIDOrHost(p, []string{"openrouter"}, []string{"openrouter.ai"})
	})
}
