package settings

import (
	"context"
	"strings"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/pkg/apperr"
)

// ConfigPatch is PATCH /api/settings body for product knobs (not users/providers).
// Omitted object sections are left untouched. Within a present section, omitted
// pointer/string fields keep their previous file values.
type ConfigPatch struct {
	Limits    *LimitsPatch     `json:"limits,omitempty"`
	LLM       *LLMGlobalsPatch `json:"llm,omitempty"`
	Context   *ContextPatch    `json:"context,omitempty"`
	Docs      *DocsPatch       `json:"docs,omitempty"`
	WebSearch *WebSearchPatch  `json:"web_search,omitempty"`
	MCP       *MCPPatch        `json:"mcp,omitempty"`
}

type LimitsPatch struct {
	MaxToolRounds     *int `json:"max_tool_rounds"`
	SpeakFloorTTLSec  *int `json:"speak_floor_ttl_sec"`
	LockTTLSec        *int `json:"lock_ttl_sec"`
	TurnJobTimeoutSec *int `json:"turn_job_timeout_sec"`
}

type LLMGlobalsPatch struct {
	Strategy           *string  `json:"strategy"`
	ActiveProvider     *string  `json:"active_provider"`
	Stream             *bool    `json:"stream"`
	Vision             *string  `json:"vision"`
	Effort             *string  `json:"effort"`
	TotalAttemptBudget *int     `json:"total_attempt_budget"`
	RetryBaseDelayMS   *int     `json:"retry_base_delay_ms"`
	RetryMaxDelayMS    *int     `json:"retry_max_delay_ms"`
	RetryJitter        *float64 `json:"retry_jitter"`
}

type ContextPatch struct {
	CompactionEnabled *bool `json:"compaction_enabled"`
	MaxInputTokens    *int  `json:"max_input_tokens"`
	ReserveTokens     *int  `json:"reserve_tokens"`
	RecentTurns       *int  `json:"recent_turns"`
	SummaryMaxChars   *int  `json:"summary_max_chars"`
}

type DocsPatch struct {
	TopK         *int     `json:"top_k"`
	MinScore     *float64 `json:"min_score"`
	FuzzyEnabled *bool    `json:"fuzzy_enabled"`
	AppID        *string  `json:"app_id"`
}

type WebSearchPatch struct {
	// GitHubToken: nil = keep; non-nil (including "") = replace stored secret.
	GitHubToken *string `json:"github_token"`
}

type MCPPatch struct {
	Enabled           *bool              `json:"enabled"`
	ConnectTimeoutSec *int               `json:"connect_timeout_sec"`
	CallTimeoutSec    *int               `json:"call_timeout_sec"`
	Servers           *[]MCPServerPublic `json:"servers"`
}

// PatchConfig updates product knobs in storage/config.json and hot-reloads runtime.
func (s *Service) PatchConfig(ctx context.Context, patch ConfigPatch) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	if err := applyConfigPatch(&doc, patch); err != nil {
		return Snapshot{}, err
	}
	if err := s.persistLocked(ctx, doc); err != nil {
		return Snapshot{}, err
	}
	s.reloadLocked(doc)
	rt := config.ApplySettingsFile(s.envCfg, doc)
	return Snapshot{
		Source:     "file",
		ConfigPath: s.store.Path(),
		Users:      doc.Users,
		LLM:        enrichLLMPublic(publicLLM(rt, doc, s.providers), rt),
		Limits:     publicLimits(rt),
		Context:    publicContext(rt),
		Docs:       publicDocs(rt),
		WebSearch:  publicWebSearch(rt),
		MCP:        publicMCP(rt),
	}, nil
}

func applyConfigPatch(doc *entity.SettingsFile, patch ConfigPatch) error {
	if patch.Limits != nil {
		if err := applyLimitsPatch(&doc.Limits, *patch.Limits); err != nil {
			return err
		}
	}
	if patch.LLM != nil {
		if err := applyLLMGlobalsPatch(&doc.LLM, *patch.LLM); err != nil {
			return err
		}
	}
	if patch.Context != nil {
		if err := applyContextPatch(&doc.Context, *patch.Context); err != nil {
			return err
		}
	}
	if patch.Docs != nil {
		if err := applyDocsPatch(&doc.Docs, *patch.Docs); err != nil {
			return err
		}
	}
	if patch.WebSearch != nil && patch.WebSearch.GitHubToken != nil {
		doc.WebSearch.GitHubToken = strings.TrimSpace(*patch.WebSearch.GitHubToken)
	}
	if patch.MCP != nil {
		if err := applyMCPPatch(&doc.MCP, *patch.MCP); err != nil {
			return err
		}
	}
	return nil
}

func applyLimitsPatch(dst *entity.SettingsLimits, p LimitsPatch) error {
	if p.MaxToolRounds != nil {
		if *p.MaxToolRounds < 1 {
			return apperr.Validation("limits.max_tool_rounds must be >= 1")
		}
		dst.MaxToolRounds = intPtr(*p.MaxToolRounds)
	}
	if p.SpeakFloorTTLSec != nil {
		if *p.SpeakFloorTTLSec < 1 {
			return apperr.Validation("limits.speak_floor_ttl_sec must be >= 1")
		}
		dst.SpeakFloorTTLSec = intPtr(*p.SpeakFloorTTLSec)
	}
	if p.LockTTLSec != nil {
		if *p.LockTTLSec < 1 {
			return apperr.Validation("limits.lock_ttl_sec must be >= 1")
		}
		dst.LockTTLSec = intPtr(*p.LockTTLSec)
	}
	if p.TurnJobTimeoutSec != nil {
		if *p.TurnJobTimeoutSec < 1 {
			return apperr.Validation("limits.turn_job_timeout_sec must be >= 1")
		}
		dst.TurnJobTimeoutSec = intPtr(*p.TurnJobTimeoutSec)
	}
	return nil
}

func applyLLMGlobalsPatch(dst *entity.SettingsLLM, p LLMGlobalsPatch) error {
	if p.Strategy != nil {
		s := strings.ToLower(strings.TrimSpace(*p.Strategy))
		switch s {
		case "failover", "round_robin", "switch":
			dst.Strategy = s
		default:
			return apperr.Validation("llm.strategy must be failover|round_robin|switch")
		}
	}
	if p.ActiveProvider != nil {
		dst.ActiveProvider = strings.ToUpper(strings.TrimSpace(*p.ActiveProvider))
	}
	if p.Stream != nil {
		dst.Stream = boolPtr(*p.Stream)
	}
	if p.Vision != nil {
		v := config.ParseVisionMode(*p.Vision)
		dst.Vision = v
	}
	if p.Effort != nil {
		e := config.ParseEffortMode(*p.Effort)
		dst.Effort = e
	}
	if p.TotalAttemptBudget != nil {
		if *p.TotalAttemptBudget < 1 {
			return apperr.Validation("llm.total_attempt_budget must be >= 1")
		}
		dst.TotalAttemptBudget = intPtr(*p.TotalAttemptBudget)
	}
	if p.RetryBaseDelayMS != nil {
		if *p.RetryBaseDelayMS < 1 {
			return apperr.Validation("llm.retry_base_delay_ms must be >= 1")
		}
		dst.RetryBaseDelayMS = intPtr(*p.RetryBaseDelayMS)
	}
	if p.RetryMaxDelayMS != nil {
		if *p.RetryMaxDelayMS < 1 {
			return apperr.Validation("llm.retry_max_delay_ms must be >= 1")
		}
		dst.RetryMaxDelayMS = intPtr(*p.RetryMaxDelayMS)
	}
	if p.RetryJitter != nil {
		if *p.RetryJitter < 0 || *p.RetryJitter > 1 {
			return apperr.Validation("llm.retry_jitter must be between 0 and 1")
		}
		dst.RetryJitter = floatPtr(*p.RetryJitter)
	}
	return nil
}

func applyContextPatch(dst *entity.SettingsContext, p ContextPatch) error {
	if p.CompactionEnabled != nil {
		dst.CompactionEnabled = boolPtr(*p.CompactionEnabled)
	}
	if p.MaxInputTokens != nil {
		if *p.MaxInputTokens < 1 {
			return apperr.Validation("context.max_input_tokens must be >= 1")
		}
		dst.MaxInputTokens = intPtr(*p.MaxInputTokens)
	}
	if p.ReserveTokens != nil {
		if *p.ReserveTokens < 0 {
			return apperr.Validation("context.reserve_tokens must be >= 0")
		}
		dst.ReserveTokens = intPtr(*p.ReserveTokens)
	}
	if p.RecentTurns != nil {
		if *p.RecentTurns < 1 {
			return apperr.Validation("context.recent_turns must be >= 1")
		}
		dst.RecentTurns = intPtr(*p.RecentTurns)
	}
	if p.SummaryMaxChars != nil {
		if *p.SummaryMaxChars < 1 {
			return apperr.Validation("context.summary_max_chars must be >= 1")
		}
		dst.SummaryMaxChars = intPtr(*p.SummaryMaxChars)
	}
	return nil
}

func applyDocsPatch(dst *entity.SettingsDocs, p DocsPatch) error {
	if p.TopK != nil {
		if *p.TopK < 1 {
			return apperr.Validation("docs.top_k must be >= 1")
		}
		dst.TopK = intPtr(*p.TopK)
	}
	if p.MinScore != nil {
		if *p.MinScore < 0 || *p.MinScore > 1 {
			return apperr.Validation("docs.min_score must be between 0 and 1")
		}
		dst.MinScore = floatPtr(*p.MinScore)
	}
	if p.FuzzyEnabled != nil {
		dst.FuzzyEnabled = boolPtr(*p.FuzzyEnabled)
	}
	if p.AppID != nil {
		dst.AppID = strings.TrimSpace(*p.AppID)
	}
	return nil
}

func applyMCPPatch(dst *entity.SettingsMCP, p MCPPatch) error {
	if p.Enabled != nil {
		dst.Enabled = boolPtr(*p.Enabled)
	}
	if p.ConnectTimeoutSec != nil {
		if *p.ConnectTimeoutSec < 1 {
			return apperr.Validation("mcp.connect_timeout_sec must be >= 1")
		}
		dst.ConnectTimeoutSec = *p.ConnectTimeoutSec
	}
	if p.CallTimeoutSec != nil {
		if *p.CallTimeoutSec < 1 {
			return apperr.Validation("mcp.call_timeout_sec must be >= 1")
		}
		dst.CallTimeoutSec = *p.CallTimeoutSec
	}
	if p.Servers != nil {
		dst.Servers = mcpServersFromPublic(*p.Servers)
	}
	return nil
}
