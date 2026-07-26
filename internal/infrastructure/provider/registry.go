// Package provider contains the provider-family registry. Concrete provider
// adapters are registered by cmd/app so settings remains independent from
// infrastructure implementations.
package provider

import (
	"fmt"
	"strings"

	"buatpostingan/internal/domain/entity"
)

// Adapter owns metadata, matching, and defaults for one provider family.
type Adapter interface {
	Definition() entity.ProviderDefinition
	Matches(entity.SettingsProvider) bool
	Normalize(entity.SettingsProvider) (entity.SettingsProvider, error)
}

type Registry struct {
	ordered []Adapter
	byType  map[string]Adapter
}

func NewRegistry(adapters ...Adapter) (*Registry, error) {
	r := &Registry{ordered: make([]Adapter, 0, len(adapters)), byType: make(map[string]Adapter, len(adapters))}
	for _, adapter := range adapters {
		if adapter == nil {
			return nil, fmt.Errorf("provider adapter is nil")
		}
		kind := strings.ToLower(strings.TrimSpace(adapter.Definition().Type))
		if kind == "" {
			return nil, fmt.Errorf("provider adapter type is required")
		}
		if _, exists := r.byType[kind]; exists {
			return nil, fmt.Errorf("duplicate provider adapter %q", kind)
		}
		r.ordered = append(r.ordered, adapter)
		r.byType[kind] = adapter
	}
	return r, nil
}

func (r *Registry) List() []entity.ProviderDefinition {
	out := make([]entity.ProviderDefinition, 0, len(r.ordered))
	for _, adapter := range r.ordered {
		def := adapter.Definition()
		if def.HideFromCatalog {
			continue
		}
		out = append(out, def)
	}
	return out
}

func (r *Registry) Infer(p entity.SettingsProvider) string {
	if kind := strings.ToLower(strings.TrimSpace(p.Type)); kind != "" {
		if _, ok := r.byType[kind]; ok {
			return kind
		}
	}
	for _, adapter := range r.ordered {
		if adapter.Definition().Type == "openai-compatible" {
			continue
		}
		if adapter.Matches(p) {
			return adapter.Definition().Type
		}
	}
	if _, ok := r.byType["openai-compatible"]; ok {
		return "openai-compatible"
	}
	return ""
}

func (r *Registry) Normalize(p entity.SettingsProvider) (entity.SettingsProvider, error) {
	kind := r.Infer(p)
	adapter, ok := r.byType[kind]
	if !ok {
		return entity.SettingsProvider{}, fmt.Errorf("unsupported provider type %q", p.Type)
	}
	if !adapter.Definition().Configurable {
		note := strings.TrimSpace(adapter.Definition().Note)
		if note == "" {
			note = "provider is not configurable"
		}
		return entity.SettingsProvider{}, fmt.Errorf("%s", note)
	}
	p.Type = kind
	return adapter.Normalize(p)
}
