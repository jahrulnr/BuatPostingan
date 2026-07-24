package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"buatpostingan/internal/config"
)

func clearLLMEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"BP_HTTP_ADDR",
		"BP_WEB_ROOT",
		"BP_STORAGE_ROOT",
		"BP_DOCS_ROOT",
		"BP_PROMPTS_ROOT",
		"BP_TOOLS_ROOT",
		"BP_MAX_TOOL_ROUNDS",
		"BP_SPEAK_FLOOR_TTL_SEC",
		"BP_LOCK_TTL_SEC",
		"BP_TURN_RATE_LIMIT_PER_MIN",
		"BP_TURN_JOB_TIMEOUT_SEC",
		"BP_LLM_STUB",
		"BP_LLM_STREAM",
		"BP_LLM_STRATEGY",
		"BP_LLM_ACTIVE_PROVIDER",
		"BP_LLM_TOTAL_ATTEMPT_BUDGET",
		"BP_LLM_CIRCUIT_FAILURE_THRESHOLD",
		"BP_LLM_CIRCUIT_COOLDOWN_SEC",
		"BP_LLM_RETRY_STATUSES",
		"BP_LLM_PROVIDERS",
		"BP_LLM_OPENROUTER_API_KEY",
		"BP_LLM_OPENROUTER_BASE_URL",
		"BP_LLM_OPENROUTER_MODEL",
		"BP_LLM_OPENROUTER_API",
		"BP_LLM_OPENROUTER_ENABLED",
		"BP_LLM_OPENROUTER_MAX_ATTEMPTS",
		"BP_LLM_OPENROUTER_CONTEXT_WINDOW",
		"BP_LLM_OPENROUTER_MAX_OUTPUT_TOKENS",
		"BP_LLM_OPENROUTER_MAX_INPUT_TOKENS",
		"BP_LLM_OPENROUTER_TIMEOUT_SEC",
		"BP_LLM_OPENROUTER_WEIGHT",
		"BP_LLM_LOCAL_API_KEY",
		"BP_LLM_LOCAL_ENABLED",
		"BP_LLM_TIMEOUT_SEC",
		"BP_LLM_CONTEXT_WINDOW",
		"BP_LLM_MAX_OUTPUT_TOKENS",
		"BP_CONTEXT_COMPACTION_ENABLED",
		"BP_CONTEXT_MAX_INPUT_TOKENS",
		"BP_CONTEXT_RESERVE_TOKENS",
		"BP_CONTEXT_RECENT_TURNS",
		"BP_CONTEXT_SUMMARY_MAX_CHARS",
		"BP_DOCS_TOP_K",
		"BP_DOCS_MIN_SCORE",
		"BP_DOCS_FUZZY_ENABLED",
		"BP_DOCS_APP_ID",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearLLMEnv(t)
	cfg := config.Load()
	if cfg.HTTPAddr != ":8080" || cfg.WebRoot != "web" || cfg.StorageRoot != "storage/webchat" {
		t.Fatalf("paths %+v", cfg)
	}
	if cfg.WriteEnabled {
		t.Fatal("WriteEnabled must stay false")
	}
	if cfg.MaxToolRounds != 8 || cfg.SpeakFloorTTL != 600 || cfg.LockTTL != 300 {
		t.Fatalf("ttls %+v", cfg)
	}
	if cfg.TurnRateLimitPerMin != 10 || cfg.TurnJobTimeoutSec != 120 {
		t.Fatalf("rate/job %+v", cfg)
	}
	if !cfg.LLMStub {
		t.Fatal("expected stub when no provider key")
	}
	if !cfg.LLMStream {
		t.Fatal("LLMStream should default true")
	}
	if cfg.LLMStrategy != "failover" || cfg.LLMActiveProvider != "OPENROUTER" {
		t.Fatalf("llm %+v %q", cfg.LLMStrategy, cfg.LLMActiveProvider)
	}
	if len(cfg.LLMRetryStatuses) < 3 || cfg.DocsTopK != 5 || cfg.DocsMinScore != 0.5 {
		t.Fatalf("retry/docs %+v", cfg)
	}
	if cfg.DocsAppID != "buatpostingan" || !cfg.DocsFuzzyEnabled {
		t.Fatalf("docs meta %+v", cfg)
	}
	p, ok := cfg.LLMProviders["OPENROUTER"]
	if !ok || p.API != "responses" || !p.Enabled {
		t.Fatalf("provider %+v", p)
	}
}

func TestLoad_BPOverrides(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_HTTP_ADDR", ":9090")
	t.Setenv("BP_MAX_TOOL_ROUNDS", "3")
	t.Setenv("BP_DOCS_MIN_SCORE", "0.9")
	t.Setenv("BP_LLM_STUB", "false")
	t.Setenv("BP_LLM_OPENROUTER_API_KEY", "sk-test")
	cfg := config.Load()
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("addr %q", cfg.HTTPAddr)
	}
	if cfg.MaxToolRounds != 3 {
		t.Fatalf("rounds %d", cfg.MaxToolRounds)
	}
	if cfg.DocsMinScore != 0.9 {
		t.Fatalf("score %v", cfg.DocsMinScore)
	}
	if cfg.LLMStub {
		t.Fatal("BP stub false should apply")
	}
}

func TestLoad_BPEnv(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_HTTP_ADDR", ":6060")
	t.Setenv("BP_STORAGE_ROOT", "tmp/storage")
	t.Setenv("BP_LLM_STRATEGY", "ROUND_ROBIN")
	t.Setenv("BP_CONTEXT_COMPACTION_ENABLED", "yes")
	t.Setenv("BP_DOCS_FUZZY_ENABLED", "off")
	t.Setenv("BP_DOCS_APP_ID", "kit")
	cfg := config.Load()
	if cfg.HTTPAddr != ":6060" || cfg.StorageRoot != "tmp/storage" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.LLMStrategy != "round_robin" {
		t.Fatalf("strategy %q", cfg.LLMStrategy)
	}
	if !cfg.ContextCompactionEnabled || cfg.DocsFuzzyEnabled || cfg.DocsAppID != "kit" {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoad_InvalidStrategyFallsBack(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_LLM_STRATEGY", "chaos")
	cfg := config.Load()
	if cfg.LLMStrategy != "failover" {
		t.Fatalf("got %q", cfg.LLMStrategy)
	}
}

func TestLoad_StubDefaultWithKey(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_LLM_OPENROUTER_API_KEY", "sk-live")
	cfg := config.Load()
	if cfg.LLMStub {
		t.Fatal("expected stub=false when key present and stub unset")
	}
}

func TestLoad_LLMStream(t *testing.T) {
	clearLLMEnv(t)
	cfg := config.Load()
	if !cfg.LLMStream {
		t.Fatal("default stream true")
	}
	t.Setenv("BP_LLM_STREAM", "false")
	cfg = config.Load()
	if cfg.LLMStream {
		t.Fatal("BP_LLM_STREAM=false should disable")
	}
	t.Setenv("BP_LLM_STREAM", "0")
	cfg = config.Load()
	if cfg.LLMStream {
		t.Fatal("BP_LLM_STREAM=0 should disable")
	}
}

func TestLoad_ProvidersNormalization(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_LLM_PROVIDERS", "OPENROUTER, ,LOCAL")
	t.Setenv("BP_LLM_OPENROUTER_API", "weird")
	t.Setenv("BP_LLM_OPENROUTER_MAX_ATTEMPTS", "0")
	t.Setenv("BP_LLM_OPENROUTER_BASE_URL", "https://example.com/v1/")
	t.Setenv("BP_LLM_OPENROUTER_CONTEXT_WINDOW", "2000")
	t.Setenv("BP_LLM_OPENROUTER_MAX_OUTPUT_TOKENS", "1500")
	t.Setenv("BP_LLM_OPENROUTER_MAX_INPUT_TOKENS", "99999")
	t.Setenv("BP_LLM_LOCAL_ENABLED", "false")
	t.Setenv("BP_LLM_LOCAL_API_KEY", "k")
	t.Setenv("BP_LLM_ACTIVE_PROVIDER", "")
	cfg := config.Load()
	or := cfg.LLMProviders["OPENROUTER"]
	if or.API != "responses" {
		t.Fatalf("api %q", or.API)
	}
	if or.MaxAttempts != 1 {
		t.Fatalf("attempts %d", or.MaxAttempts)
	}
	if or.BaseURL != "https://example.com/v1" {
		t.Fatalf("base %q", or.BaseURL)
	}
	// budget = 2000-1500-512 = -12 → clamped to 1000; MaxInputTokens capped to budget
	if or.MaxInputTokens != 1000 {
		t.Fatalf("max input %d", or.MaxInputTokens)
	}
	if cfg.LLMActiveProvider != "OPENROUTER" {
		// LOCAL disabled; first enabled sorted id is OPENROUTER
		t.Fatalf("active %q providers=%v", cfg.LLMActiveProvider, cfg.LLMProviders)
	}
	if cfg.LLMProviders["LOCAL"].Enabled {
		t.Fatal("LOCAL should be disabled")
	}
}

func TestLoad_ExplicitActiveProvider(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_LLM_PROVIDERS", "OPENROUTER,LOCAL")
	t.Setenv("BP_LLM_ACTIVE_PROVIDER", "local")
	t.Setenv("BP_LLM_LOCAL_API_KEY", "x")
	cfg := config.Load()
	if cfg.LLMActiveProvider != "LOCAL" {
		t.Fatalf("got %q", cfg.LLMActiveProvider)
	}
}

func TestLoad_InvalidIntFloatBoolIgnored(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_MAX_TOOL_ROUNDS", "nope")
	t.Setenv("BP_DOCS_MIN_SCORE", "nope")
	t.Setenv("BP_LLM_STUB", "maybe")
	t.Setenv("BP_LLM_RETRY_STATUSES", "429,abc,-1,0,500")
	cfg := config.Load()
	if cfg.MaxToolRounds != 8 || cfg.DocsMinScore != 0.5 {
		t.Fatalf("defaults lost: %d %v", cfg.MaxToolRounds, cfg.DocsMinScore)
	}
	if !cfg.LLMStub {
		t.Fatal("invalid bool should keep fallback stub=true")
	}
	if len(cfg.LLMRetryStatuses) != 2 || cfg.LLMRetryStatuses[0] != 429 || cfg.LLMRetryStatuses[1] != 500 {
		t.Fatalf("%v", cfg.LLMRetryStatuses)
	}
}

func TestLoad_DotEnvFillsUnset(t *testing.T) {
	clearLLMEnv(t)
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "# comment\n\nexport BP_HTTP_ADDR=:5555\nBP_DOCS_APP_ID=\"from-dotenv\"\nBP_DOCS_TOP_K='7'\nbadline\n=novalue\nBP_LLM_STUB=1\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// Pre-set wins over dotenv
	t.Setenv("BP_DOCS_APP_ID", "preset")
	cfg := config.Load()
	if cfg.HTTPAddr != ":5555" {
		t.Fatalf("dotenv addr %q", cfg.HTTPAddr)
	}
	if cfg.DocsAppID != "preset" {
		t.Fatalf("preset should win, got %q", cfg.DocsAppID)
	}
	if cfg.DocsTopK != 7 {
		t.Fatalf("topK %d", cfg.DocsTopK)
	}
	if !cfg.LLMStub {
		t.Fatal("expected stub from dotenv")
	}
}

func TestLoad_ChatAPIPreserved(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_LLM_OPENROUTER_API", "chat")
	cfg := config.Load()
	if cfg.LLMProviders["OPENROUTER"].API != "chat" {
		t.Fatalf("%+v", cfg.LLMProviders["OPENROUTER"])
	}
}
