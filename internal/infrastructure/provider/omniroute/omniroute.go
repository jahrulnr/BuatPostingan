package omniroute

import (
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/infrastructure/provider"
)

func New() provider.Adapter {
	def := entity.ProviderDefinition{
		Type: "omniroute", Name: "OmniRoute",
		Description: "Local multi-provider AI gateway with OpenAI-compatible endpoints.",
		AuthType:    "local", API: "responses", BaseURL: "http://127.0.0.1:20128/v1",
		Prefix: "omniroute", Icon: "OM", Accent: "#ef476f", Configurable: true,
		APIKeyOptional: true,
	}
	return provider.NewStatic(def, func(p entity.SettingsProvider) bool {
		return provider.MatchIDOrHost(p, []string{"omniroute"}, []string{":20128"})
	})
}
