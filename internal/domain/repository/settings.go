package repository

import (
	"context"

	"buatpostingan/internal/domain/entity"
)

// SettingsStore loads/saves the JSON app config file.
type SettingsStore interface {
	Path() string
	Exists() bool
	Load(ctx context.Context) (entity.SettingsFile, error)
	Save(ctx context.Context, doc entity.SettingsFile) error
}
