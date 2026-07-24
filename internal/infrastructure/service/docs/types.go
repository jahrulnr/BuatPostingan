package docs

// Options configures Index scoring and identity.
type Options struct {
	AppID         string
	TopK          int     // used when Search topK <= 0
	MinScore      float64 // keyword score floor (config docs_min_score)
	DisableFuzzy  bool    // when false (default), fuzzy matching is enabled
	SkipAutoBuild bool    // when false (default), NewIndex builds once if index missing
}

func (o Options) withDefaults() Options {
	if o.AppID == "" {
		o.AppID = DefaultAppID
	}
	if o.TopK <= 0 {
		o.TopK = DefaultTopK
	}
	if o.MinScore <= 0 {
		o.MinScore = DefaultMinScore
	}
	return o
}

func (o Options) fuzzyEnabled() bool {
	return !o.DisableFuzzy
}

// Filters optional language/domain constraints (used by search_docs tool).
type Filters struct {
	Language string
	Domain   string
}

// Hit is one ranked chunk from Search / SearchHits.
type Hit struct {
	Path     string  `json:"path"`
	Title    string  `json:"title"`
	Heading  string  `json:"heading"`
	ChunkID  string  `json:"chunk_id"`
	Language string  `json:"language"`
	Domain   string  `json:"domain"`
	AppID    string  `json:"app_id"`
	Excerpt  string  `json:"excerpt"`
	Score    float64 `json:"score"`
}

type chunk struct {
	ID      string `json:"id"`
	Heading string `json:"heading"`
	Text    string `json:"text"`
}

type document struct {
	Path     string   `json:"path"`
	Title    string   `json:"title"`
	Headings []string `json:"headings"`
	Text     string   `json:"text"`
	Language string   `json:"language"`
	Domain   string   `json:"domain"`
	AppID    string   `json:"app_id"`
	Chunks   []chunk  `json:"chunks"`
	MTime    int64    `json:"mtime"`
}

type indexFile struct {
	BuiltAt       float64    `json:"built_at"`
	DocsRoot      string     `json:"docs_root"`
	AppID         string     `json:"app_id"`
	DocumentCount int        `json:"document_count"`
	Documents     []document `json:"documents"`
}

type statusMeta struct {
	Status        string  `json:"status"`
	At            float64 `json:"at,omitempty"`
	Message       string  `json:"message,omitempty"`
	DocumentCount int     `json:"document_count,omitempty"`
}
