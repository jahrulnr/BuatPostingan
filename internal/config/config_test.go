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
		"BP_SKILLS_ROOT",
		"BP_LLM_STUB",
		"BP_LLM_RETRY_STATUSES",
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
	if cfg.LLMVision != "auto" {
		t.Fatalf("LLMVision default %q", cfg.LLMVision)
	}
	if cfg.LLMEffort != "auto" {
		t.Fatalf("LLMEffort default %q", cfg.LLMEffort)
	}
	if cfg.LLMStrategy != "failover" || cfg.LLMActiveProvider != "" {
		t.Fatalf("llm %+v %q", cfg.LLMStrategy, cfg.LLMActiveProvider)
	}
	if len(cfg.LLMRetryStatuses) < 3 || cfg.DocsTopK != 5 || cfg.DocsMinScore != 0.5 {
		t.Fatalf("retry/docs %+v", cfg)
	}
	if cfg.DocsAppID != "buatpostingan" || !cfg.DocsFuzzyEnabled {
		t.Fatalf("docs meta %+v", cfg)
	}
	if cfg.LLMProviders != nil {
		t.Fatalf("providers should be nil from env, got %+v", cfg.LLMProviders)
	}
}

func TestLoad_BPOverrides(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_HTTP_ADDR", ":9090")
	t.Setenv("BP_LLM_STUB", "false")
	cfg := config.Load()
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("addr %q", cfg.HTTPAddr)
	}
	if cfg.LLMStub {
		t.Fatal("BP stub false should apply")
	}
}

func TestLoad_BPEnv(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_HTTP_ADDR", ":6060")
	t.Setenv("BP_STORAGE_ROOT", "tmp/storage")
	cfg := config.Load()
	if cfg.HTTPAddr != ":6060" || cfg.StorageRoot != "tmp/storage" {
		t.Fatalf("%+v", cfg)
	}
}

func TestParseVisionMode(t *testing.T) {
	if config.ParseVisionMode("1") != "on" || config.ParseVisionMode("false") != "off" {
		t.Fatal("aliases")
	}
}

func TestParseEffortMode(t *testing.T) {
	if config.ParseEffortMode("x-high") != "xhigh" || config.ParseEffortMode("maximum") != "max" {
		t.Fatal("aliases")
	}
	if config.ParseEffortMode("") != "auto" {
		t.Fatal("empty → auto")
	}
}

func TestLoad_InvalidIntFloatBoolIgnored(t *testing.T) {
	clearLLMEnv(t)
	t.Setenv("BP_LLM_STUB", "maybe")
	t.Setenv("BP_LLM_RETRY_STATUSES", "429,abc,-1,0,500")
	cfg := config.Load()
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
	content := "# comment\n\nexport BP_HTTP_ADDR=:5555\nbadline\n=novalue\nBP_LLM_STUB=1\n"
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

	cfg := config.Load()
	if cfg.HTTPAddr != ":5555" {
		t.Fatalf("dotenv addr %q", cfg.HTTPAddr)
	}
	if !cfg.LLMStub {
		t.Fatal("expected stub from dotenv")
	}
}

func TestLoad_ChatAPIPreserved(t *testing.T) {
	clearLLMEnv(t)
	cfg := config.Load()
	if cfg.LLMProviders != nil {
		t.Fatal("providers should be nil from env")
	}
}
