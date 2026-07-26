package service

import (
	"context"

	"buatpostingan/internal/domain/entity"
)

// ProviderRegistry owns provider-family discovery, defaults, and validation.
// Settings depends on this interface; concrete provider adapters are injected
// by cmd/app.
type ProviderRegistry interface {
	List() []entity.ProviderDefinition
	Infer(entity.SettingsProvider) string
	Normalize(entity.SettingsProvider) (entity.SettingsProvider, error)
}

// ProviderModelImporter fetches the list of model ids exposed by an OpenAI- or
// Anthropic-compatible provider at its /models endpoint.
type ProviderModelImporter interface {
	ImportModels(ctx context.Context, provider entity.SettingsProvider) ([]entity.SettingsModel, error)
}
