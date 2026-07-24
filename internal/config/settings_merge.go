package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"buatpostingan/internal/domain/entity"
)

// ConfigPath returns BP_CONFIG_PATH or default beside StorageRoot.
func (c Config) ConfigPath() string {
	override := strings.TrimSpace(os.Getenv("BP_CONFIG_PATH"))
	root := c.StorageRoot
	if root == "" {
		root = "storage/webchat"
	}
	if override != "" {
		return override
	}
	return filepath.Clean(filepath.Join(filepath.Dir(root), "config.json"))
}

// ApplySettingsFile overlays file LLM providers onto env-based Config when
// providers are non-empty. Users are not part of Config (settings usecase only).
func ApplySettingsFile(base Config, doc entity.SettingsFile) Config {
	out := base
	if len(doc.LLM.Providers) == 0 {
		return out
	}
	providers := make(map[string]LLMProvider, len(doc.LLM.Providers))
	for _, sp := range doc.LLM.Providers {
		p := FileProviderToRuntime(sp)
		if p.ID == "" {
			continue
		}
		providers[p.ID] = p
	}
	if len(providers) == 0 {
		return out
	}
	out.LLMProviders = providers
	lists := make(map[string][]string, len(doc.LLM.Providers))
	for _, sp := range doc.LLM.Providers {
		id := strings.ToUpper(strings.TrimSpace(sp.ID))
		if id == "" {
			continue
		}
		var ids []string
		for _, m := range sp.Models {
			mid := strings.TrimSpace(m.ID)
			if mid == "" {
				continue
			}
			ids = append(ids, mid)
		}
		lists[id] = ids
	}
	out.LLMModelLists = lists

	if s := strings.ToLower(strings.TrimSpace(doc.LLM.Strategy)); s != "" {
		switch s {
		case "failover", "round_robin", "switch":
			out.LLMStrategy = s
		}
	}
	if ap := strings.ToUpper(strings.TrimSpace(doc.LLM.ActiveProvider)); ap != "" {
		out.LLMActiveProvider = ap
	} else {
		ids := make([]string, 0, len(providers))
		for id := range providers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out.LLMActiveProvider = ""
		for _, id := range ids {
			if providers[id].Enabled {
				out.LLMActiveProvider = id
				break
			}
		}
	}
	if doc.LLM.Stub != nil {
		out.LLMStub = *doc.LLM.Stub
	} else {
		anyKey := false
		for _, p := range providers {
			if strings.TrimSpace(p.APIKey) != "" {
				anyKey = true
				break
			}
		}
		out.LLMStub = !anyKey
	}
	return out
}

// FileProviderToRuntime maps settings JSON → LLMProvider slot.
func FileProviderToRuntime(sp entity.SettingsProvider) LLMProvider {
	id := strings.ToUpper(strings.TrimSpace(sp.ID))
	api := strings.ToLower(strings.TrimSpace(sp.API))
	if api != "chat" && api != "responses" {
		api = "responses"
	}
	model := ""
	if len(sp.Models) > 0 {
		model = strings.TrimSpace(sp.Models[0].ID)
	}
	timeout := sp.TimeoutSec
	if timeout < 1 {
		timeout = 60
	}
	maxAttempts := sp.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	weight := sp.Weight
	if weight < 1 {
		weight = 1
	}
	key := strings.TrimSpace(sp.APIKey)
	if key == "" && len(sp.APIKeys) > 0 {
		key = strings.TrimSpace(sp.APIKeys[0])
	}
	return LLMProvider{
		ID:          id,
		BaseURL:     strings.TrimRight(strings.TrimSpace(sp.BaseURL), "/"),
		APIKey:      key,
		Model:       model,
		API:         api,
		TimeoutSec:  timeout,
		MaxAttempts: maxAttempts,
		Weight:      weight,
		Enabled:     sp.Enabled,
		// Keep sensible defaults for sizing (env-era defaults).
		ContextWindow:   131072,
		MaxOutputTokens: 4096,
		MaxInputTokens:  12000,
	}
}

// RuntimeProvidersToFile converts env map → settings providers (for seeding).
func RuntimeProvidersToFile(providers map[string]LLMProvider) []entity.SettingsProvider {
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]entity.SettingsProvider, 0, len(ids))
	for _, id := range ids {
		p := providers[id]
		models := []entity.SettingsModel{}
		if strings.TrimSpace(p.Model) != "" {
			models = append(models, entity.SettingsModel{ID: p.Model})
		}
		out = append(out, entity.SettingsProvider{
			ID:          p.ID,
			Name:        p.ID,
			Prefix:      strings.ToLower(p.ID),
			API:         p.API,
			BaseURL:     p.BaseURL,
			APIKey:      p.APIKey,
			APIKeys:     nil,
			Enabled:     p.Enabled,
			Models:      models,
			TimeoutSec:  p.TimeoutSec,
			MaxAttempts: p.MaxAttempts,
			Weight:      p.Weight,
		})
	}
	return out
}

// MaskAPIKey returns a display string; never the full secret.
func MaskAPIKey(key string) (set bool, masked string) {
	k := strings.TrimSpace(key)
	if k == "" {
		return false, ""
	}
	if len(k) <= 4 {
		return true, "••••"
	}
	return true, "••••…" + k[len(k)-4:]
}
