package llm

import (
	"sync"

	"buatpostingan/internal/config"
)

// ConfigReloader receives merged app config after settings writes.
type ConfigReloader interface {
	Reload(cfg config.Config)
}

// Runtime hot-reloads Catalog + Router (+ Client) + optional worker after settings writes.
type Runtime struct {
	mu      sync.Mutex
	router  *Router
	catalog *Catalog
	vision  *VisionPolicy
	effort  *EffortPolicy
	extra   []ConfigReloader
}

func NewRuntime(router *Router, catalog *Catalog, vision *VisionPolicy, effort *EffortPolicy, extra ...ConfigReloader) *Runtime {
	return &Runtime{router: router, catalog: catalog, vision: vision, effort: effort, extra: extra}
}

func (rt *Runtime) Reload(app config.Config) {
	if rt == nil {
		return
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	llmCfg := FromApp(app)
	if rt.router != nil {
		rt.router.Reload(llmCfg)
	}
	if rt.catalog != nil {
		rt.catalog.Reload(app)
	}
	if rt.vision != nil {
		rt.vision.Reload(llmCfg)
	}
	if rt.effort != nil {
		rt.effort.Reload(llmCfg)
	}
	for _, x := range rt.extra {
		if x != nil {
			x.Reload(app)
		}
	}
}
