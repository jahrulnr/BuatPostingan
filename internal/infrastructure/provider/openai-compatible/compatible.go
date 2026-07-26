package openaicompatible

import (
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/infrastructure/provider"
)

func New() provider.Adapter {
	def := entity.ProviderDefinition{
		Type: "openai-compatible", Name: "OpenAI Compatible",
		Description: "Custom OpenAI-compatible endpoint hosted by you or another vendor.",
		AuthType:    "api_key", API: "chat", Prefix: "compatible",
		Icon: "OC", Accent: "#64748b", Configurable: true,
		HideFromCatalog: true,
	}
	return provider.NewStatic(def, func(entity.SettingsProvider) bool { return true })
}
