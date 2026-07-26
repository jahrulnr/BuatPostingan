package openai

import (
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/infrastructure/provider"
)

func New() provider.Adapter {
	def := entity.ProviderDefinition{
		Type: "openai", Name: "OpenAI",
		Description: "Official OpenAI API using the Responses API by default.",
		AuthType:    "api_key", API: "responses", BaseURL: "https://api.openai.com/v1",
		Prefix: "openai", Icon: "OA", Accent: "#10a37f", Configurable: true,
	}
	return provider.NewStatic(def, func(p entity.SettingsProvider) bool {
		return provider.MatchIDOrHost(p, []string{"openai"}, []string{"api.openai.com"})
	})
}
