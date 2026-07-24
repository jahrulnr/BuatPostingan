package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	httpdelivery "buatpostingan/delivery/http"
	webchatusecase "buatpostingan/internal/usecase/webchat"
	"buatpostingan/internal/config"
	"buatpostingan/internal/infrastructure/ratelimit"
	"buatpostingan/internal/infrastructure/repository/attachments"
	"buatpostingan/internal/infrastructure/repository/jsonl"
	"buatpostingan/internal/infrastructure/service/docs"
	"buatpostingan/internal/infrastructure/service/llm"
	"buatpostingan/internal/infrastructure/service/tools"
	"buatpostingan/internal/infrastructure/sse"
	"buatpostingan/internal/infrastructure/worker"
	"buatpostingan/internal/pkg/redact"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	if err := os.MkdirAll(cfg.StorageRoot, 0o775); err != nil {
		log.Fatalf("storage root: %v", err)
	}
	for _, sub := range []string{"threads", "interrupt", "rl", "llm", "attachments"} {
		_ = os.MkdirAll(filepath.Join(cfg.StorageRoot, sub), 0o775)
	}

	store := jsonl.NewStore(cfg.StorageRoot)
	locks := jsonl.NewLock(cfg.StorageRoot, cfg.LockTTL)
	intr := jsonl.NewInterrupt(cfg.StorageRoot)
	floor := jsonl.NewSpeakFloor(store, cfg.SpeakFloorTTL)
	rl := ratelimit.NewTurnLimiter(cfg.StorageRoot, cfg.TurnRateLimitPerMin)
	red := redact.New()
	attStore, err := attachments.NewStore(cfg.StorageRoot, 0)
	if err != nil {
		log.Fatalf("attachments: %v", err)
	}

	docsIndex, err := docs.NewIndex(cfg.DocsRoot, cfg.StorageRoot, docs.Options{
		AppID:        cfg.DocsAppID,
		TopK:         cfg.DocsTopK,
		MinScore:     cfg.DocsMinScore,
		DisableFuzzy: !cfg.DocsFuzzyEnabled,
	})
	if err != nil {
		log.Fatalf("docs index: %v", err)
	}
	if err := docsIndex.Reindex(ctx); err != nil {
		log.Printf("docs reindex warning: %v", err)
	}
	gate, gerr := docsIndex.Gate(ctx)
	if gerr != nil {
		log.Printf("docs gate error: %v", gerr)
	} else {
		log.Printf("docs gate: usable=%v status=%s docs=%d msg=%s",
			gate.Usable, gate.Status, gate.DocumentCount, gate.Message)
	}

	llmCfg := llm.FromApp(cfg)
	llmClient := llm.NewClient(llmCfg)
	llmRouter := llm.NewRouter(llmCfg, llmClient)
	visionPolicy := llm.NewVisionPolicy(llmCfg)
	effortPolicy := llm.NewEffortPolicy(llmCfg)
	modelCatalog := llm.NewCatalog(cfg, visionPolicy, effortPolicy)

	reg, err := tools.NewRegistry(cfg.ToolsRoot, docsIndex, tools.Options{
		TopK:        cfg.DocsTopK,
		Attachments: attStore,
		Vision:      visionPolicy,
		// FSRoot empty: list_dir/read_file/grep have full host FS access (local-dev).
	})
	if err != nil {
		log.Fatalf("tools registry: %v", err)
	}

	tw := worker.New(worker.Deps{
		Config:      cfg,
		Store:       store,
		Locks:       locks,
		Interrupt:   intr,
		Tools:       reg,
		Docs:        docsIndex,
		LLM:         llmRouter,
		Attachments: attStore,
		Vision:      visionPolicy,
	})
	events := sse.NewStreamer(store)

	log.Printf("webchat ready: llm_stub=%v vision=%s effort=%s strategy=%s active=%s providers=%d",
		cfg.LLMStub, visionPolicy.Mode(), effortPolicy.Mode(), cfg.LLMStrategy, cfg.LLMActiveProvider, len(cfg.LLMProviders))

	uc := webchatusecase.NewService(webchatusecase.Deps{
		Threads:     store,
		Locks:       locks,
		Interrupt:   intr,
		Floor:       floor,
		RateLimit:   rl,
		Redactor:    red,
		Docs:        docsIndex,
		Events:      events,
		Worker:      tw,
		Attachments: attStore,
		Models:      modelCatalog,
	})

	srv := httpdelivery.NewServer(cfg, uc)
	if err := httpdelivery.ListenAndServe(srv); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}
