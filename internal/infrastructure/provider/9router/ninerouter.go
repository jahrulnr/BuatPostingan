package ninerouter

import (
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/infrastructure/provider"
)

func New() provider.Adapter {
	def := entity.ProviderDefinition{
		Type: "9router", Name: "9Router",
		Description: "Local 9Router gateway using its OpenAI-compatible endpoint.",
		AuthType:    "local", API: "chat", BaseURL: "http://127.0.0.1:20128/v1",
		Prefix: "9router", Icon: "9R", Accent: "#16c784", Configurable: true,
		APIKeyOptional: true,
	}
	return provider.NewStatic(def, func(p entity.SettingsProvider) bool {
		return provider.MatchIDOrHost(p, []string{"9router", "9_router"}, []string{":20130"})
	})
}
