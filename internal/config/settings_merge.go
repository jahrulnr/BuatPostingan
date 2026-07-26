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

// ApplySettingsFile overlays file settings onto the base Config from Load().
//
// Merge policy (JSON is SoT for product knobs; base provides hardcoded defaults):
//   - providers: file providers are the sole source — Load() sets nil. When
//     JSON has providers, they populate the map; when JSON omits providers,
//     the map stays nil (stub mode via BP_LLM_STUB).
//   - llm globals (stream/vision/effort/retry): pointer-set wins; omit
//     keeps the hardcoded default.
//   - limits / context / docs / web_search: pointer-set wins; omit keeps the
//     hardcoded default. This keeps old/missing files working.
//   - users are not part of Config (settings usecase only).
//
// Missing or pre-dating files still work: every field uses a pointer (or
// non-zero string) so an absent key leaves the hardcoded default untouched.
func ApplySettingsFile(base Config, doc entity.SettingsFile) Config {
	out := applyMCPFromFile(base, doc)
	out = applyLLMGlobalsFromFile(out, doc.LLM)
	out = applyLimitsFromFile(out, doc.Limits)
	out = applyContextFromFile(out, doc.Context)
	out = applyDocsFromFile(out, doc.Docs)
	if ws := strings.TrimSpace(doc.WebSearch.GitHubToken); ws != "" {
		out.GitHubToken = ws
	}

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
	modelsByProvider := make(map[string][]LLMModel, len(doc.LLM.Providers))
	for _, sp := range doc.LLM.Providers {
		id := strings.ToUpper(strings.TrimSpace(sp.ID))
		if id == "" {
			continue
		}
		var ids []string
		var models []LLMModel
		seen := make(map[string]struct{}, len(sp.Models))
		for _, m := range sp.Models {
			mid := strings.TrimSpace(m.ID)
			if mid == "" {
				continue
			}
			if _, ok := seen[mid]; ok {
				continue
			}
			seen[mid] = struct{}{}
			ids = append(ids, mid)
			models = append(models, LLMModel{
				ID:          mid,
				Label:       strings.TrimSpace(m.Label),
				Task:        strings.ToLower(strings.TrimSpace(m.Task)),
				OutputModes: append([]string(nil), m.OutputModes...),
			})
		}
		lists[id] = ids
		modelsByProvider[id] = models
	}
	out.LLMModelLists = lists
	out.LLMModels = modelsByProvider

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
	anyUsableProvider := false
	for _, p := range providers {
		if p.Enabled && (strings.TrimSpace(p.APIKey) != "" || p.APIKeyOptional) {
			anyUsableProvider = true
			break
		}
	}
	out.LLMStub = !anyUsableProvider
	return out
}

// applyLLMGlobalsFromFile overlays LLM global knobs (stream/vision/effort/
// retry backoff). Pointer-set wins; omit keeps hardcoded default.
// Applied even when providers are absent (so globals move to JSON
// independently).
func applyLLMGlobalsFromFile(base Config, llm entity.SettingsLLM) Config {
	out := base
	if llm.Stream != nil {
		out.LLMStream = *llm.Stream
	}
	if v := strings.TrimSpace(llm.Vision); v != "" {
		out.LLMVision = ParseVisionMode(v)
	}
	if e := strings.TrimSpace(llm.Effort); e != "" {
		out.LLMEffort = ParseEffortMode(e)
	}
	if llm.TotalAttemptBudget != nil && *llm.TotalAttemptBudget > 0 {
		out.LLMTotalAttemptBudget = *llm.TotalAttemptBudget
	}
	if llm.RetryBaseDelayMS != nil && *llm.RetryBaseDelayMS > 0 {
		out.LLMRetryBaseDelayMS = *llm.RetryBaseDelayMS
	}
	if llm.RetryMaxDelayMS != nil && *llm.RetryMaxDelayMS > 0 {
		out.LLMRetryMaxDelayMS = *llm.RetryMaxDelayMS
	}
	if llm.RetryJitter != nil && *llm.RetryJitter >= 0 {
		out.LLMRetryJitter = *llm.RetryJitter
	}
	if s := strings.ToLower(strings.TrimSpace(llm.Strategy)); s != "" {
		switch s {
		case "failover", "round_robin", "switch":
			out.LLMStrategy = s
		}
	}
	if ap := strings.ToUpper(strings.TrimSpace(llm.ActiveProvider)); ap != "" {
		out.LLMActiveProvider = ap
	}
	return out
}

func applyLimitsFromFile(base Config, limits entity.SettingsLimits) Config {
	out := base
	if limits.MaxToolRounds != nil && *limits.MaxToolRounds > 0 {
		out.MaxToolRounds = *limits.MaxToolRounds
	}
	if limits.SpeakFloorTTLSec != nil && *limits.SpeakFloorTTLSec > 0 {
		out.SpeakFloorTTL = *limits.SpeakFloorTTLSec
	}
	if limits.LockTTLSec != nil && *limits.LockTTLSec > 0 {
		out.LockTTL = *limits.LockTTLSec
	}
	if limits.TurnJobTimeoutSec != nil && *limits.TurnJobTimeoutSec > 0 {
		out.TurnJobTimeoutSec = *limits.TurnJobTimeoutSec
	}
	return out
}

func applyContextFromFile(base Config, ctx entity.SettingsContext) Config {
	out := base
	if ctx.CompactionEnabled != nil {
		out.ContextCompactionEnabled = *ctx.CompactionEnabled
	}
	if ctx.MaxInputTokens != nil && *ctx.MaxInputTokens > 0 {
		out.ContextMaxInputTokens = *ctx.MaxInputTokens
	}
	if ctx.ReserveTokens != nil && *ctx.ReserveTokens > 0 {
		out.ContextReserveTokens = *ctx.ReserveTokens
	}
	if ctx.RecentTurns != nil && *ctx.RecentTurns > 0 {
		out.ContextRecentTurns = *ctx.RecentTurns
	}
	if ctx.SummaryMaxChars != nil && *ctx.SummaryMaxChars > 0 {
		out.ContextSummaryMaxChars = *ctx.SummaryMaxChars
	}
	return out
}

func applyDocsFromFile(base Config, docs entity.SettingsDocs) Config {
	out := base
	if docs.TopK != nil && *docs.TopK > 0 {
		out.DocsTopK = *docs.TopK
	}
	if docs.MinScore != nil && *docs.MinScore >= 0 {
		out.DocsMinScore = *docs.MinScore
	}
	if docs.FuzzyEnabled != nil {
		out.DocsFuzzyEnabled = *docs.FuzzyEnabled
	}
	if app := strings.TrimSpace(docs.AppID); app != "" {
		out.DocsAppID = app
	}
	return out
}

func applyMCPFromFile(base Config, doc entity.SettingsFile) Config {
	out := base
	if doc.MCP.Enabled != nil {
		out.MCPEnabled = *doc.MCP.Enabled
	}
	if doc.MCP.ConnectTimeoutSec > 0 {
		out.MCPConnectTimeoutSec = doc.MCP.ConnectTimeoutSec
	}
	if doc.MCP.CallTimeoutSec > 0 {
		out.MCPCallTimeoutSec = doc.MCP.CallTimeoutSec
	}
	if doc.MCP.Servers == nil {
		return out
	}
	servers := make([]MCPServer, 0, len(doc.MCP.Servers))
	for _, s := range doc.MCP.Servers {
		id := strings.TrimSpace(s.ID)
		if id == "" {
			continue
		}
		transport := strings.ToLower(strings.TrimSpace(s.Transport))
		if transport == "" {
			transport = "stdio"
		}
		env := map[string]string{}
		for k, v := range s.Env {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			env[k] = v
		}
		servers = append(servers, MCPServer{
			ID:             id,
			Transport:      transport,
			Command:        strings.TrimSpace(s.Command),
			Args:           append([]string(nil), s.Args...),
			Env:            env,
			URL:            strings.TrimSpace(s.URL),
			Enabled:        s.Enabled,
			Trusted:        s.Trusted,
			AllowTools:     append([]string(nil), s.AllowTools...),
			DenyTools:      append([]string(nil), s.DenyTools...),
			AllowMutations: s.AllowMutations,
		})
	}
	out.MCPServers = servers
	return out
}

// DefaultLocalDevMCP returns the sample echo server block used to seed new
// settings documents (not applied at process start unless written).
func DefaultLocalDevMCP() entity.SettingsMCP {
	enabled := true
	return entity.SettingsMCP{
		Enabled:           &enabled,
		ConnectTimeoutSec: 15,
		CallTimeoutSec:    30,
		Servers: []entity.SettingsMCPServer{{
			ID:             "echo",
			Transport:      "stdio",
			Command:        "./bin/mcp-echo",
			Args:           []string{},
			Env:            map[string]string{},
			Enabled:        true,
			Trusted:        true,
			AllowTools:     []string{"echo"},
			DenyTools:      []string{},
			AllowMutations: false,
		}},
	}
}

// DefaultSeedFile returns a fresh SettingsFile snapshot derived from Load()
// defaults plus env providers (when present) and the local-dev MCP sample.
//
// Used both by appconfig.Store.EnsureSeeded() at boot (writes the file when
// missing) and by the settings usecase for in-memory seeding. Omitting
// providers keeps the file in stub mode until the operator adds one.
func DefaultSeedFile(base Config) entity.SettingsFile {
	maxToolRounds := base.MaxToolRounds
	speakFloorTTL := base.SpeakFloorTTL
	lockTTL := base.LockTTL
	turnTimeout := base.TurnJobTimeoutSec
	stream := base.LLMStream
	totalAttempts := base.LLMTotalAttemptBudget
	retryBase := base.LLMRetryBaseDelayMS
	retryMax := base.LLMRetryMaxDelayMS
	retryJitter := base.LLMRetryJitter
	compaction := base.ContextCompactionEnabled
	maxInput := base.ContextMaxInputTokens
	reserve := base.ContextReserveTokens
	recentTurns := base.ContextRecentTurns
	summaryMax := base.ContextSummaryMaxChars
	topK := base.DocsTopK
	minScore := base.DocsMinScore
	fuzzy := base.DocsFuzzyEnabled

	return entity.SettingsFile{
		Version: 1,
		Users:   []entity.SettingsUser{{ID: "usr_owner", Name: "Owner", Role: "owner"}},
		Limits: entity.SettingsLimits{
			MaxToolRounds:     &maxToolRounds,
			SpeakFloorTTLSec:  &speakFloorTTL,
			LockTTLSec:        &lockTTL,
			TurnJobTimeoutSec: &turnTimeout,
		},
		LLM: entity.SettingsLLM{
			Strategy:           base.LLMStrategy,
			ActiveProvider:     base.LLMActiveProvider,
			Stream:             &stream,
			Vision:             base.LLMVision,
			Effort:             base.LLMEffort,
			TotalAttemptBudget: &totalAttempts,
			RetryBaseDelayMS:   &retryBase,
			RetryMaxDelayMS:    &retryMax,
			RetryJitter:        &retryJitter,
			Providers:          RuntimeProvidersToFile(base.LLMProviders),
		},
		Context: entity.SettingsContext{
			CompactionEnabled: &compaction,
			MaxInputTokens:    &maxInput,
			ReserveTokens:     &reserve,
			RecentTurns:       &recentTurns,
			SummaryMaxChars:   &summaryMax,
		},
		Docs: entity.SettingsDocs{
			TopK:         &topK,
			MinScore:     &minScore,
			FuzzyEnabled: &fuzzy,
			AppID:        base.DocsAppID,
		},
		WebSearch: entity.SettingsWebSearch{GitHubToken: base.GitHubToken},
		MCP:       DefaultLocalDevMCP(),
	}
}

// FileProviderToRuntime maps settings JSON → LLMProvider slot.
func FileProviderToRuntime(sp entity.SettingsProvider) LLMProvider {
	id := strings.ToUpper(strings.TrimSpace(sp.ID))
	api := strings.ToLower(strings.TrimSpace(sp.API))
	if api != "chat" && api != "responses" && api != "messages" {
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
		Type:           strings.ToLower(strings.TrimSpace(sp.Type)),
		ID:             id,
		BaseURL:        strings.TrimRight(strings.TrimSpace(sp.BaseURL), "/"),
		APIKey:         key,
		Model:          model,
		API:            api,
		APIKeyOptional: sp.APIKeyOptional,
		TimeoutSec:     timeout,
		MaxAttempts:    maxAttempts,
		Weight:         weight,
		Enabled:        sp.Enabled,
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
			Type:           p.Type,
			ID:             p.ID,
			Name:           p.ID,
			Prefix:         strings.ToLower(p.ID),
			API:            p.API,
			BaseURL:        p.BaseURL,
			APIKey:         p.APIKey,
			APIKeys:        nil,
			APIKeyOptional: p.APIKeyOptional,
			Enabled:        p.Enabled,
			Models:         models,
			TimeoutSec:     p.TimeoutSec,
			MaxAttempts:    p.MaxAttempts,
			Weight:         p.Weight,
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
