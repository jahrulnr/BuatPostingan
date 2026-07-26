package main

import (
	"context"
	"os"
	"path/filepath"

	httpdelivery "buatpostingan/delivery/http"
	"buatpostingan/internal/config"
	"buatpostingan/internal/infrastructure/auth"
	"buatpostingan/internal/infrastructure/repository/appconfig"
	"buatpostingan/internal/infrastructure/repository/attachments"
	"buatpostingan/internal/infrastructure/repository/jsonl"
	"buatpostingan/internal/infrastructure/service/docs"
	"buatpostingan/internal/infrastructure/service/llm"
	"buatpostingan/internal/infrastructure/service/mcp"
	"buatpostingan/internal/infrastructure/service/tools"
	"buatpostingan/internal/infrastructure/sse"
	"buatpostingan/internal/infrastructure/worker"
	"buatpostingan/internal/pkg/logging"
	"buatpostingan/internal/pkg/redact"
	settingsuc "buatpostingan/internal/usecase/settings"
	webchatusecase "buatpostingan/internal/usecase/webchat"
)

func main() {
	envCfg := config.Load()
	ctx := logging.SystemContext(context.Background())

	cfgPath := envCfg.ConfigPath()
	settingsStore := appconfig.NewStore(cfgPath)
	cfg := envCfg
	if _, created, err := settingsStore.EnsureSeeded(ctx, envCfg); err != nil {
		logging.Warn(ctx, "config seed warning", "path", cfgPath, "err", err.Error())
	} else if created {
		logging.Info(ctx, "config seeded", "path", cfgPath, "note", "generated from struct defaults; edit to add providers/keys")
	}
	if settingsStore.Exists() {
		if doc, err := settingsStore.Load(ctx); err != nil {
			logging.Warn(ctx, "config.json load warning", "err", err.Error())
		} else {
			cfg = config.ApplySettingsFile(envCfg, doc)
			logging.Info(ctx, "config merged", "path", cfgPath, "providers", len(cfg.LLMProviders), "source", "file")
		}
	} else {
		logging.Info(ctx, "config bootstrap", "path", cfgPath, "note", "config.json unavailable — stub mode")
	}

	if err := os.MkdirAll(cfg.StorageRoot, 0o775); err != nil {
		logging.Error(ctx, "storage.root", err)
		os.Exit(1)
	}
	authDBPath := cfg.AuthDBPath
	if !filepath.IsAbs(authDBPath) {
		absAuthDBPath, err := filepath.Abs(authDBPath)
		if err != nil {
			logging.Error(ctx, "auth.path", err)
			os.Exit(1)
		}
		authDBPath = absAuthDBPath
	}
	if err := os.MkdirAll(filepath.Dir(authDBPath), 0o775); err != nil {
		logging.Error(ctx, "auth.root", err)
		os.Exit(1)
	}
	authStore, err := auth.NewStore(authDBPath)
	if err != nil {
		logging.Error(ctx, "auth.store", err)
		os.Exit(1)
	}
	defer authStore.Close()
	if cfg.AuthUsername != "" || cfg.AuthPassword != "" {
		created, bootstrapErr := authStore.Bootstrap(ctx, cfg.AuthUsername, cfg.AuthPassword)
		if bootstrapErr != nil {
			logging.Error(ctx, "auth.bootstrap", bootstrapErr)
			os.Exit(1)
		}
		if created {
			logging.Info(ctx, "auth bootstrap user created", "username", cfg.AuthUsername)
		}
	}
	pagesRoot := filepath.Join(filepath.Dir(cfg.StorageRoot), "pages")
	if err := os.MkdirAll(pagesRoot, 0o775); err != nil {
		logging.Error(ctx, "pages.root", err)
		os.Exit(1)
	}
	for _, sub := range []string{"threads", "interrupt", "llm", "attachments"} {
		_ = os.MkdirAll(filepath.Join(cfg.StorageRoot, sub), 0o775)
	}

	hub := sse.NewHub()
	rawStore := jsonl.NewStore(cfg.StorageRoot)
	store := &sse.NotifyingStore{Inner: rawStore, Hub: hub}
	locks := jsonl.NewLock(cfg.StorageRoot, cfg.LockTTL)
	intr := jsonl.NewInterrupt(cfg.StorageRoot)
	floor := jsonl.NewSpeakFloor(store, cfg.SpeakFloorTTL)
	red := redact.New()
	attStore, err := attachments.NewStore(cfg.StorageRoot, 0)
	if err != nil {
		logging.Error(ctx, "attachments", err)
		os.Exit(1)
	}

	docsIndex, err := docs.NewIndex(cfg.DocsRoot, cfg.StorageRoot, docs.Options{
		AppID:        cfg.DocsAppID,
		TopK:         cfg.DocsTopK,
		MinScore:     cfg.DocsMinScore,
		DisableFuzzy: !cfg.DocsFuzzyEnabled,
	})
	if err != nil {
		logging.Error(ctx, "docs.index", err)
		os.Exit(1)
	}
	if err := docsIndex.Reindex(ctx); err != nil {
		logging.Warn(ctx, "docs reindex warning", "err", err.Error())
	}
	gate, gerr := docsIndex.Gate(ctx)
	if gerr != nil {
		logging.Error(ctx, "docs.gate", gerr)
	} else {
		logging.Info(ctx, "docs gate",
			"usable", gate.Usable,
			"status", gate.Status,
			"docs", gate.DocumentCount,
			"msg", gate.Message,
		)
	}

	llmCfg := llm.FromApp(cfg)
	llmClient := llm.NewClient(llmCfg)
	llmRouter := llm.NewRouter(llmCfg, llmClient)
	visionPolicy := llm.NewVisionPolicy(llmCfg)
	effortPolicy := llm.NewEffortPolicy(llmCfg)
	modelCatalog := llm.NewCatalog(cfg, visionPolicy, effortPolicy)

	mcpMgr := mcp.NewManager(cfg)
	logging.Info(ctx, "mcp manager",
		"enabled", cfg.MCPEnabled,
		"servers", len(cfg.MCPServers),
	)
	if cfg.MCPEnabled && len(cfg.MCPServers) == 0 {
		logging.Warn(ctx, "mcp enabled with no servers",
			"hint", "add mcp.servers to storage/config.json; make mcp-echo; restart make be",
			"path", cfgPath,
		)
	}

	reg, err := tools.NewRegistry(cfg.ToolsRoot, docsIndex, tools.Options{
		TopK:        cfg.DocsTopK,
		Attachments: attStore,
		Vision:      visionPolicy,
		SkillsRoot:  cfg.SkillsRoot,
		PagesRoot:   pagesRoot,
		GitHubToken: cfg.GitHubToken,
		MCP:         mcpMgr,
		// FSRoot empty: list_dir/read_file/grep have full host FS access (local-dev).
	})
	if err != nil {
		logging.Error(ctx, "tools.registry", err)
		os.Exit(1)
	}
	defer func() { _ = mcpMgr.Close() }()

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
		Hub:         hub,
	})
	llmRuntime := llm.NewRuntime(llmRouter, modelCatalog, visionPolicy, effortPolicy, tw)
	modelImporter := llm.NewModelImporter()
	settingsSvc := settingsuc.NewService(settingsStore, envCfg, llmRuntime, modelImporter)
	events := sse.NewStreamer(store, hub)

	logging.Info(ctx, "webchat ready",
		"llm_stub", cfg.LLMStub,
		"vision", visionPolicy.Mode(),
		"effort", effortPolicy.Mode(),
		"strategy", cfg.LLMStrategy,
		"active", cfg.LLMActiveProvider,
		"providers", len(cfg.LLMProviders),
	)

	uc := webchatusecase.NewService(webchatusecase.Deps{
		Threads:       store,
		Locks:         locks,
		Interrupt:     intr,
		Floor:         floor,
		Redactor:      red,
		Docs:          docsIndex,
		Events:        events,
		Worker:        tw,
		Attachments:   attStore,
		Models:        modelCatalog,
		Pages:         reg,
		WorkspaceRoot: cfg.WorkspaceRoot,
	})

	srv := httpdelivery.NewServer(cfg, uc, settingsSvc, authStore)
	if err := httpdelivery.ListenAndServe(srv); err != nil {
		logging.Error(ctx, "http.server", err)
		os.Exit(1)
	}
}
