package service

import (
	"context"

	"buatpostingan/internal/domain/entity"
)

// ProviderModelImporter fetches the list of model ids exposed by an OpenAI- or
// Anthropic-compatible provider at its /models endpoint.
type ProviderModelImporter interface {
	ImportModels(ctx context.Context, provider entity.SettingsProvider) ([]entity.SettingsModel, error)
}
