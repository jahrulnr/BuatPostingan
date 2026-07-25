package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/infrastructure/service/docs"
	mcpmgr "buatpostingan/internal/infrastructure/service/mcp"
)

var _ service.ToolRegistry = (*Registry)(nil)

// DocsIndex is the subset of docs.Index used by search_docs.
type DocsIndex interface {
	Gate(ctx context.Context) (entity.DocsIndexGate, error)
	Usable() bool
	SearchHits(ctx context.Context, query string, topK int, filters docs.Filters) ([]docs.Hit, error)
}

// VisionPixelsGate reports whether multimodal pixels are allowed for the active model.
type VisionPixelsGate interface {
	AllowPixels(ctx context.Context) bool
}

// Options configures Registry defaults.
type Options struct {
	TopK        int // default for search_docs when top_k omitted
	Attachments repository.AttachmentStore
	// WebSearch overrides the default searchwire-backed searcher (tests).
	WebSearch WebSearcher
	// FetchClient overrides the SSRF-safe HTTP client for web_fetch (tests).
	FetchClient *http.Client
	// Vision optional; when nil, read_image treats bytes under the size cap as available.
	Vision VisionPixelsGate
	// FSRoot is an optional base for relative paths in list_dir / read_file / grep.
	// Empty = unrestricted real filesystem (relative → process cwd; absolute incl. "/").
	// Non-empty is a relative-path default only — not a sandbox jail. Prefer empty for local dev.
	FSRoot string
	// SkillsRoot is the jail root for list_skills / read_skill (default BP_SKILLS_ROOT).
	// Missing/empty → soft empty catalog / skills_unavailable (not a process crash).
	SkillsRoot string
	// GitHubToken is the optional web_search rate-limit token (env: BP_GITHUB_TOKEN
	// / GITHUB_TOKEN; also configurable via config.json → web_search.github_token).
	GitHubToken string
	// MCP is optional; when nil/disabled, list_mcp_tools / call_mcp_tool soft-fail.
	MCP MCPClient
}

// MCPClient is the subset of the MCP manager used by meta-tools.
type MCPClient interface {
	Enabled() bool
	ServerIDs() []string
	ListTools(ctx context.Context, serverID string) (tools []mcpmgr.ToolInfo, serverErrors map[string]string)
	CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (mcpmgr.CallResult, error)
}

// Registry loads *.tool.json schemas and executes allowlisted tools.
type Registry struct {
	toolsRoot   string
	index       DocsIndex
	fs          *workspaceFS
	skills      *skillsFS
	attachments repository.AttachmentStore
	webSearch   WebSearcher
	fetchClient *http.Client
	vision      VisionPixelsGate
	topK        int
	githubToken string
	mcp         MCPClient
	schemas     map[string]map[string]any // name -> raw tool json
}

// NewRegistry constructs a ToolRegistry.
// toolsRoot should contain search_docs.tool.json, list_dir.tool.json, etc.
// Filesystem tools use opts.FSRoot (empty = full host FS for local development).
func NewRegistry(toolsRoot string, index DocsIndex, opts Options) (*Registry, error) {
	if index == nil {
		return nil, errf("tools: DocsIndex required")
	}
	fs, err := newWorkspaceFS(opts.FSRoot)
	if err != nil {
		return nil, err
	}
	skills, err := newSkillsFS(opts.SkillsRoot)
	if err != nil {
		return nil, err
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = DefaultTopK
	}
	r := &Registry{
		toolsRoot:   toolsRoot,
		index:       index,
		fs:          fs,
		skills:      skills,
		attachments: opts.Attachments,
		webSearch:   opts.WebSearch,
		fetchClient: opts.FetchClient,
		vision:      opts.Vision,
		topK:        topK,
		githubToken: opts.GitHubToken,
		mcp:         opts.MCP,
		schemas:     map[string]map[string]any{},
	}
	if err := r.loadSchemas(); err != nil {
		return nil, err
	}
	return r, nil
}

func errf(msg string) error { return fmt.Errorf("%s", msg) }

func (r *Registry) loadSchemas() error {
	names := Allowlist
	for _, name := range names {
		path := filepath.Join(r.toolsRoot, name+".tool.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			continue
		}
		r.schemas[name] = schema
	}
	return nil
}

// Schemas implements service.ToolRegistry (OpenAI chat-completions function shape).
func (r *Registry) Schemas(ctx context.Context) ([]map[string]any, error) {
	_ = ctx
	names := Allowlist
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		schema, ok := r.schemas[name]
		if !ok {
			continue
		}
		out = append(out, toOpenAI(schema))
	}
	return out, nil
}

func toOpenAI(schema map[string]any) map[string]any {
	params, _ := schema["parameters"].(map[string]any)
	if params == nil {
		params = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	name, _ := schema["name"].(string)
	desc, _ := schema["description"].(string)
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        name,
			"description": desc,
			"parameters":  params,
		},
	}
}

// Execute implements service.ToolRegistry.
func (r *Registry) Execute(ctx context.Context, call service.ToolCall) (service.ToolEnvelope, error) {
	started := time.Now()
	name := call.Name
	if !r.allowed(name) {
		return r.fail("tool_not_allowed", "Tool not allowlisted", name, started), nil
	}

	args := call.Arguments
	if args == nil {
		args = map[string]any{}
	}
	params := r.parametersFor(name)
	args = healArgs(args, params)

	switch name {
	case "search_docs":
		env := r.execSearchDocs(ctx, args)
		env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		return env, nil
	case "list_dir", "read_file", "grep":
		env, err := r.execFS(ctx, name, args)
		if err != nil {
			return r.fail("invalid_path", err.Error(), name, started), nil
		}
		env.Tool = name
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		return env, nil
	case "write_file", "edit_file", "delete_file":
		env, err := r.execFS(ctx, name, args)
		if err != nil {
			return r.fail("invalid_path", err.Error(), name, started), nil
		}
		env.Tool = name
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		return env, nil
	case "exec":
		env, err := r.execShell(ctx, args)
		if err != nil {
			return r.fail("exec_error", err.Error(), name, started), nil
		}
		env.Tool = name
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		return env, nil
	case "read_attachment":
		env := r.execReadAttachment(ctx, args)
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		return env, nil
	case "read_image":
		env := r.execReadImage(ctx, args)
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		return env, nil
	case "web_search":
		env := r.execWebSearch(ctx, args)
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		return env, nil
	case "web_fetch":
		env := r.execWebFetch(ctx, args)
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		return env, nil
	case "list_skills":
		env := r.execListSkills(args)
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		if _, ok := env.Meta["took_ms"]; !ok {
			env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		}
		return env, nil
	case "read_skill":
		env := r.execReadSkill(args)
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		if _, ok := env.Meta["took_ms"]; !ok {
			env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		}
		return env, nil
	case "list_mcp_tools":
		env := r.execListMCPTools(ctx, args)
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		if _, ok := env.Meta["took_ms"]; !ok {
			env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		}
		return env, nil
	case "call_mcp_tool":
		env := r.execCallMCPTool(ctx, args)
		if env.Meta == nil {
			env.Meta = map[string]any{}
		}
		if _, ok := env.Meta["took_ms"]; !ok {
			env.Meta["took_ms"] = int(time.Since(started).Milliseconds())
		}
		return env, nil
	default:
		return r.fail("unknown_tool", "Unknown tool", name, started), nil
	}
}

func (r *Registry) allowed(name string) bool {
	for _, n := range Allowlist {
		if n == name {
			return true
		}
	}
	return false
}

func (r *Registry) parametersFor(name string) map[string]any {
	schema, ok := r.schemas[name]
	if !ok {
		return nil
	}
	params, _ := schema["parameters"].(map[string]any)
	return params
}

func (r *Registry) execSearchDocs(ctx context.Context, args map[string]any) service.ToolEnvelope {
	query := strings.TrimSpace(asString(args["query"]))
	if query == "" {
		gate, _ := r.index.Gate(ctx)
		return service.ToolEnvelope{
			OK:   false,
			Tool: "search_docs",
			Data: nil,
			Error: map[string]any{
				"code":    "validation",
				"message": "query required",
			},
			Meta: map[string]any{
				"truncated":         false,
				"index_ready":       gate.Usable,
				"index_status":      gate.Status,
				"data_is_untrusted": true,
			},
		}
	}

	topK := asInt(args["top_k"], r.topK)
	if topK < 1 {
		topK = DefaultTopK
	}

	if !r.index.Usable() {
		gate, _ := r.index.Gate(ctx)
		return service.ToolEnvelope{
			OK:   false,
			Tool: "search_docs",
			Data: nil,
			Error: map[string]any{
				"code":    "docs_index_not_ready",
				"message": gate.Message,
			},
			Meta: map[string]any{
				"truncated":         false,
				"count":             0,
				"index_ready":       false,
				"index_status":      gate.Status,
				"data_is_untrusted": true,
			},
		}
	}

	filters := docs.Filters{}
	if lang := strings.TrimSpace(asString(args["language"])); lang != "" {
		filters.Language = lang
	}
	if domain := strings.TrimSpace(asString(args["domain"])); domain != "" {
		filters.Domain = domain
	}

	hits, err := r.index.SearchHits(ctx, query, topK, filters)
	if err != nil {
		gate, _ := r.index.Gate(ctx)
		return service.ToolEnvelope{
			OK:    false,
			Tool:  "search_docs",
			Data:  nil,
			Error: map[string]any{"code": "tool_error", "message": "search failed"},
			Meta: map[string]any{
				"truncated":         false,
				"index_ready":       gate.Usable,
				"index_status":      gate.Status,
				"data_is_untrusted": true,
			},
		}
	}

	return service.ToolEnvelope{
		OK:    true,
		Tool:  "search_docs",
		Data:  map[string]any{"chunks": hits},
		Error: nil,
		Meta: map[string]any{
			"truncated":         len(hits) >= topK,
			"count":             len(hits),
			"index_ready":       true,
			"index_status":      "ready",
			"data_is_untrusted": true,
		},
	}
}

func (r *Registry) execShell(ctx context.Context, args map[string]any) (service.ToolEnvelope, error) {
	raw, err := (&shellExec{}).run(ctx, args)
	if err != nil {
		return service.ToolEnvelope{}, err
	}
	return mapToEnvelope(raw), nil
}

func (r *Registry) execFS(ctx context.Context, name string, args map[string]any) (service.ToolEnvelope, error) {
	var raw map[string]any
	var err error
	switch name {
	case "list_dir":
		raw, err = r.fs.listDir(ctx, args)
	case "read_file":
		raw, err = r.fs.readFile(ctx, args)
	case "grep":
		raw, err = r.fs.grep(ctx, args)
	case "write_file":
		raw, err = r.fs.writeFile(ctx, args)
	case "edit_file":
		raw, err = r.fs.editFile(ctx, args)
	case "delete_file":
		raw, err = r.fs.deleteFile(ctx, args)
	default:
		return service.ToolEnvelope{}, errf("unknown")
	}
	if err != nil {
		return service.ToolEnvelope{}, err
	}
	return mapToEnvelope(raw), nil
}

func mapToEnvelope(raw map[string]any) service.ToolEnvelope {
	env := service.ToolEnvelope{
		OK:   false,
		Tool: asString(raw["tool"]),
		Data: raw["data"],
		Meta: map[string]any{},
	}
	if ok, _ := raw["ok"].(bool); ok {
		env.OK = true
	}
	if errObj, ok := raw["error"].(map[string]any); ok {
		env.Error = errObj
	}
	if meta, ok := raw["meta"].(map[string]any); ok {
		env.Meta = meta
	}
	return env
}

func (r *Registry) fail(code, message, name string, started time.Time) service.ToolEnvelope {
	return service.ToolEnvelope{
		OK:   false,
		Tool: name,
		Data: nil,
		Error: map[string]any{
			"code":    code,
			"message": message,
		},
		Meta: map[string]any{
			"truncated":         false,
			"took_ms":           int(time.Since(started).Milliseconds()),
			"data_is_untrusted": true,
		},
	}
}
