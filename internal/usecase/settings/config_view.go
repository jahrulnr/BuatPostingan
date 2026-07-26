package settings

import (
	"strings"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
)

// LimitsPublic is resolved turn-loop limits for the settings UI.
type LimitsPublic struct {
	MaxToolRounds     int `json:"max_tool_rounds"`
	SpeakFloorTTLSec  int `json:"speak_floor_ttl_sec"`
	LockTTLSec        int `json:"lock_ttl_sec"`
	TurnJobTimeoutSec int `json:"turn_job_timeout_sec"`
}

// ContextPublic is resolved context-compaction knobs.
type ContextPublic struct {
	CompactionEnabled bool `json:"compaction_enabled"`
	MaxInputTokens    int  `json:"max_input_tokens"`
	ReserveTokens     int  `json:"reserve_tokens"`
	RecentTurns       int  `json:"recent_turns"`
	SummaryMaxChars   int  `json:"summary_max_chars"`
}

// DocsPublic is resolved docs retrieval knobs.
type DocsPublic struct {
	TopK         int     `json:"top_k"`
	MinScore     float64 `json:"min_score"`
	FuzzyEnabled bool    `json:"fuzzy_enabled"`
	AppID        string  `json:"app_id"`
}

// WebSearchPublic masks github_token like provider API keys.
type WebSearchPublic struct {
	GitHubTokenSet    bool   `json:"github_token_set"`
	GitHubTokenMasked string `json:"github_token_masked,omitempty"`
}

// MCPPublic is MCP config for the settings UI (env values included — operator-owned).
type MCPPublic struct {
	Enabled           bool              `json:"enabled"`
	ConnectTimeoutSec int               `json:"connect_timeout_sec"`
	CallTimeoutSec    int               `json:"call_timeout_sec"`
	Servers           []MCPServerPublic `json:"servers"`
}

// MCPServerPublic is one MCP server row.
type MCPServerPublic struct {
	ID             string            `json:"id"`
	Transport      string            `json:"transport"`
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	URL            string            `json:"url,omitempty"`
	Enabled        bool              `json:"enabled"`
	Trusted        bool              `json:"trusted,omitempty"`
	AllowTools     []string          `json:"allow_tools,omitempty"`
	DenyTools      []string          `json:"deny_tools,omitempty"`
	AllowMutations bool              `json:"allow_mutations,omitempty"`
}

func publicLimits(rt config.Config) LimitsPublic {
	return LimitsPublic{
		MaxToolRounds:     rt.MaxToolRounds,
		SpeakFloorTTLSec:  rt.SpeakFloorTTL,
		LockTTLSec:        rt.LockTTL,
		TurnJobTimeoutSec: rt.TurnJobTimeoutSec,
	}
}

func publicContext(rt config.Config) ContextPublic {
	return ContextPublic{
		CompactionEnabled: rt.ContextCompactionEnabled,
		MaxInputTokens:    rt.ContextMaxInputTokens,
		ReserveTokens:     rt.ContextReserveTokens,
		RecentTurns:       rt.ContextRecentTurns,
		SummaryMaxChars:   rt.ContextSummaryMaxChars,
	}
}

func publicDocs(rt config.Config) DocsPublic {
	return DocsPublic{
		TopK:         rt.DocsTopK,
		MinScore:     rt.DocsMinScore,
		FuzzyEnabled: rt.DocsFuzzyEnabled,
		AppID:        rt.DocsAppID,
	}
}

func publicWebSearch(rt config.Config) WebSearchPublic {
	set, masked := config.MaskAPIKey(rt.GitHubToken)
	return WebSearchPublic{GitHubTokenSet: set, GitHubTokenMasked: masked}
}

func publicMCP(rt config.Config) MCPPublic {
	servers := make([]MCPServerPublic, 0, len(rt.MCPServers))
	for _, s := range rt.MCPServers {
		env := map[string]string{}
		for k, v := range s.Env {
			env[k] = v
		}
		servers = append(servers, MCPServerPublic{
			ID:             s.ID,
			Transport:      s.Transport,
			Command:        s.Command,
			Args:           append([]string(nil), s.Args...),
			Env:            env,
			URL:            s.URL,
			Enabled:        s.Enabled,
			Trusted:        s.Trusted,
			AllowTools:     append([]string(nil), s.AllowTools...),
			DenyTools:      append([]string(nil), s.DenyTools...),
			AllowMutations: s.AllowMutations,
		})
	}
	return MCPPublic{
		Enabled:           rt.MCPEnabled,
		ConnectTimeoutSec: rt.MCPConnectTimeoutSec,
		CallTimeoutSec:    rt.MCPCallTimeoutSec,
		Servers:           servers,
	}
}

func enrichLLMPublic(base LLMPublic, rt config.Config) LLMPublic {
	base.Stream = rt.LLMStream
	base.Vision = rt.LLMVision
	base.Effort = rt.LLMEffort
	base.TotalAttemptBudget = rt.LLMTotalAttemptBudget
	base.RetryBaseDelayMS = rt.LLMRetryBaseDelayMS
	base.RetryMaxDelayMS = rt.LLMRetryMaxDelayMS
	base.RetryJitter = rt.LLMRetryJitter
	return base
}

func intPtr(n int) *int { return &n }

func boolPtr(b bool) *bool { return &b }

func floatPtr(f float64) *float64 { return &f }

func mcpServersFromPublic(servers []MCPServerPublic) []entity.SettingsMCPServer {
	out := make([]entity.SettingsMCPServer, 0, len(servers))
	for _, s := range servers {
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
		out = append(out, entity.SettingsMCPServer{
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
	return out
}
