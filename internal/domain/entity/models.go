package entity

// ModelOption is one selectable LLM slot for the composer model picker.
type ModelOption struct {
	ID               string
	Label            string
	Provider         string
	SupportsVision   bool
	SupportedEfforts []string // empty = effort UI uses global options / omit when unsupported
	DefaultEffort    string
	Disabled         bool
}

// ModelsCatalog is the GET /api/webchat/models payload source.
type ModelsCatalog struct {
	Models         []ModelOption
	DefaultModelID string
	EffortCurrent  string   // server default from BP_LLM_EFFORT
	EffortOptions  []string // global picker options (includes auto)
	Stub           bool
}
