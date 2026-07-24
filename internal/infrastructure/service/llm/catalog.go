package llm

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/apperr"
)

// Catalog builds the model-picker list from configured providers (+ optional live enrich).
type Catalog struct {
	providers map[string]config.LLMProvider
	active    string
	effortCfg string
	stub      bool
	effort    *EffortPolicy
	vision    *VisionPolicy
}

// NewCatalog wires picker listing. stub=true returns a canned list (no API keys exposed).
func NewCatalog(app config.Config, vision *VisionPolicy, effort *EffortPolicy) *Catalog {
	return &Catalog{
		providers: app.LLMProviders,
		active:    app.LLMActiveProvider,
		effortCfg: config.ParseEffortMode(app.LLMEffort),
		stub:      app.LLMStub,
		effort:    effort,
		vision:    vision,
	}
}

var _ service.ModelCatalog = (*Catalog)(nil)

func (c *Catalog) ListModels(ctx context.Context) (entity.ModelsCatalog, error) {
	out := entity.ModelsCatalog{
		EffortCurrent: c.effortCfg,
		EffortOptions: config.EffortPickerOptions(),
		Stub:          c.stub,
		Models:        nil,
	}
	if c.stub {
		out.Models = stubModels()
		out.DefaultModelID = out.Models[0].ID
		return out, nil
	}

	ids := make([]string, 0, len(c.providers))
	for id, p := range c.providers {
		if !p.Enabled {
			continue
		}
		if strings.TrimSpace(p.Model) == "" && strings.TrimSpace(p.APIKey) == "" {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		p := c.providers[id]
		modelID := strings.TrimSpace(p.Model)
		if modelID == "" {
			modelID = id
		}
		opt := entity.ModelOption{
			ID:             modelID,
			Label:          modelLabel(modelID, id),
			Provider:       id,
			DefaultEffort:  "auto",
			SupportsVision: false,
		}
		if c.vision != nil {
			opt.SupportsVision = c.vision.SupportsImageFor(ctx, p)
		}
		if c.effort != nil {
			info := c.effort.lookup(ctx, p)
			if info.Supports {
				opt.SupportedEfforts = normalizeEffortList(info.SupportedEfforts)
				if len(opt.SupportedEfforts) == 0 {
					// Supports but no allowlist → all non-auto gateway levels.
					opt.SupportedEfforts = []string{
						EffortNone, EffortMinimal, EffortLow, EffortMedium,
						EffortHigh, EffortXHigh, EffortMax,
					}
				}
				def := strings.TrimSpace(info.DefaultEffort)
				if def != "" {
					opt.DefaultEffort = config.ParseEffortMode(def)
				} else {
					opt.DefaultEffort = EffortMedium
				}
			}
		} else if HeuristicModelSupportsEffort(modelID) {
			opt.SupportedEfforts = []string{
				EffortNone, EffortMinimal, EffortLow, EffortMedium,
				EffortHigh, EffortXHigh, EffortMax,
			}
			opt.DefaultEffort = EffortMedium
		}
		out.Models = append(out.Models, opt)
	}

	if len(out.Models) == 0 {
		out.Models = stubModels()
		out.Stub = true
		out.DefaultModelID = out.Models[0].ID
		return out, nil
	}

	out.DefaultModelID = defaultModelID(c.active, c.providers, out.Models)
	return out, nil
}

func (c *Catalog) ResolveModel(_ context.Context, modelOrProvider string) (string, error) {
	raw := strings.TrimSpace(modelOrProvider)
	if raw == "" {
		return "", nil
	}
	if c.stub {
		for _, m := range stubModels() {
			if m.ID == raw || strings.EqualFold(m.Provider, raw) {
				return m.Provider, nil
			}
		}
		return "", apperr.Validation("model not allowed")
	}

	upper := strings.ToUpper(raw)
	if p, ok := c.providers[upper]; ok {
		if !p.Enabled {
			return "", apperr.Validation("model not allowed")
		}
		return p.ID, nil
	}
	for _, id := range sortedProviderIDs(c.providers) {
		p := c.providers[id]
		if !p.Enabled {
			continue
		}
		if strings.TrimSpace(p.Model) == raw {
			return p.ID, nil
		}
	}
	return "", apperr.Validation("model not allowed")
}

func stubModels() []entity.ModelOption {
	return []entity.ModelOption{
		{
			ID:             "stub/default",
			Label:          "Stub default",
			Provider:       "STUB",
			SupportsVision: false,
			DefaultEffort:  "auto",
		},
		{
			ID:               "stub/reasoning",
			Label:            "Stub reasoning",
			Provider:         "STUB",
			SupportsVision:   false,
			SupportedEfforts: []string{EffortNone, EffortLow, EffortMedium, EffortHigh},
			DefaultEffort:    EffortMedium,
		},
		{
			ID:             "stub/vision",
			Label:          "Stub vision",
			Provider:       "STUB",
			SupportsVision: true,
			DefaultEffort:  "auto",
		},
	}
}

func modelLabel(modelID, providerID string) string {
	if modelID == "" || modelID == providerID {
		return providerID
	}
	return fmt.Sprintf("%s · %s", shortModel(modelID), providerID)
}

func shortModel(modelID string) string {
	if i := strings.LastIndex(modelID, "/"); i >= 0 && i+1 < len(modelID) {
		return modelID[i+1:]
	}
	return modelID
}

func defaultModelID(active string, providers map[string]config.LLMProvider, models []entity.ModelOption) string {
	if active != "" {
		if p, ok := providers[active]; ok && p.Enabled {
			m := strings.TrimSpace(p.Model)
			if m == "" {
				m = p.ID
			}
			for _, opt := range models {
				if opt.ID == m && opt.Provider == p.ID {
					return opt.ID
				}
			}
			for _, opt := range models {
				if opt.ID == m {
					return opt.ID
				}
			}
		}
	}
	if len(models) > 0 {
		return models[0].ID
	}
	return ""
}

func sortedProviderIDs(providers map[string]config.LLMProvider) []string {
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func normalizeEffortList(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, e := range raw {
		n, ok := config.NormalizeEffortOverride(e)
		if !ok || n == "" || n == EffortAuto {
			continue
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}
