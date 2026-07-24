// Package mcp implements an on-demand MCP client manager (stdio MVP).
//
// Official SDK: github.com/modelcontextprotocol/go-sdk/mcp
// Streamable HTTP / OAuth are reserved extension points — see docs/architecture/mcp-support.md.
package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"buatpostingan/internal/config"
	"buatpostingan/internal/pkg/logging"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolInfo is a discovered MCP tool (catalog entry for list_mcp_tools).
type ToolInfo struct {
	Server      string         `json:"server"`
	Name        string         `json:"name"`
	Namespaced  string         `json:"namespaced"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
	Mutating    bool           `json:"mutating"`
	Allowed     bool           `json:"allowed"`
}

// CallResult is a normalized tools/call outcome.
type CallResult struct {
	Server    string
	Tool      string
	IsError   bool
	Content   []map[string]any
	RawText   string
	Trusted   bool
}

// Manager connects to configured MCP servers on demand and isolates failures.
type Manager struct {
	mu               sync.Mutex
	enabled          bool
	connectTimeout   time.Duration
	callTimeout      time.Duration
	servers          map[string]config.MCPServer
	sessions         map[string]*mcpsdk.ClientSession
	client           *mcpsdk.Client
	dial             dialFunc // injectable for tests
}

type dialFunc func(ctx context.Context, srv config.MCPServer) (*mcpsdk.ClientSession, error)

// NewManager builds a manager from runtime config. Safe with zero servers.
func NewManager(cfg config.Config) *Manager {
	m := &Manager{
		enabled:        cfg.MCPEnabled,
		connectTimeout: time.Duration(cfg.MCPConnectTimeoutSec) * time.Second,
		callTimeout:    time.Duration(cfg.MCPCallTimeoutSec) * time.Second,
		servers:        map[string]config.MCPServer{},
		sessions:       map[string]*mcpsdk.ClientSession{},
		client: mcpsdk.NewClient(&mcpsdk.Implementation{
			Name:    "buatpostingan-webchat",
			Version: "1.0.0",
		}, nil),
	}
	if m.connectTimeout <= 0 {
		m.connectTimeout = 15 * time.Second
	}
	if m.callTimeout <= 0 {
		m.callTimeout = 30 * time.Second
	}
	for _, s := range cfg.MCPServers {
		id := strings.TrimSpace(s.ID)
		if id == "" {
			continue
		}
		m.servers[id] = s
	}
	m.dial = m.dialStdio
	return m
}

// Enabled reports whether MCP meta-tools should operate.
func (m *Manager) Enabled() bool {
	if m == nil {
		return false
	}
	return m.enabled
}

// ServerIDs returns configured server ids (enabled or not), sorted for stability in tests via caller.
func (m *Manager) ServerIDs() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.servers))
	for id := range m.servers {
		out = append(out, id)
	}
	return out
}

// Close shuts down all sessions.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var first error
	for id, sess := range m.sessions {
		if sess == nil {
			continue
		}
		if err := sess.Close(); err != nil && first == nil {
			first = err
		}
		delete(m.sessions, id)
	}
	return first
}

// ListTools discovers tools. Empty serverID lists all enabled servers.
// Per-server failures are isolated into skipped entries (returned via error string map).
func (m *Manager) ListTools(ctx context.Context, serverID string) (tools []ToolInfo, serverErrors map[string]string) {
	serverErrors = map[string]string{}
	if m == nil || !m.enabled {
		return nil, serverErrors
	}
	ids := m.targetIDs(serverID)
	for _, id := range ids {
		srv, ok := m.server(id)
		if !ok {
			serverErrors[id] = "server_not_found"
			continue
		}
		if !srv.Enabled {
			serverErrors[id] = "server_disabled"
			continue
		}
		listed, err := m.listOne(ctx, srv)
		if err != nil {
			serverErrors[id] = err.Error()
			logging.Warn(ctx, "mcp.list_tools failed", "server", id, "err", err.Error())
			continue
		}
		tools = append(tools, listed...)
	}
	return tools, serverErrors
}

// CallTool invokes one MCP tool after policy checks.
func (m *Manager) CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (CallResult, error) {
	empty := CallResult{Server: serverID, Tool: toolName}
	if m == nil || !m.enabled {
		return empty, fmt.Errorf("mcp_disabled")
	}
	srv, ok := m.server(serverID)
	if !ok {
		return empty, fmt.Errorf("server_not_found")
	}
	if !srv.Enabled {
		return empty, fmt.Errorf("server_disabled")
	}
	if err := CheckCallAllowed(srv, toolName, ""); err != nil {
		return empty, err
	}
	sess, err := m.session(ctx, srv)
	if err != nil {
		return empty, err
	}
	callCtx, cancel := context.WithTimeout(ctx, m.callTimeout)
	defer cancel()
	logging.Info(ctx, "mcp.call", "server", serverID, "tool", toolName, "trace_id", logging.TraceID(ctx))
	res, err := sess.CallTool(callCtx, &mcpsdk.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		// drop session; next call reconnects
		m.dropSession(serverID)
		return empty, err
	}
	out := CallResult{
		Server:  serverID,
		Tool:    toolName,
		IsError: res != nil && res.IsError,
		Trusted: srv.Trusted,
	}
	if res != nil {
		out.Content, out.RawText = flattenContent(res.Content)
	}
	return out, nil
}

func (m *Manager) listOne(ctx context.Context, srv config.MCPServer) ([]ToolInfo, error) {
	sess, err := m.session(ctx, srv)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, m.callTimeout)
	defer cancel()
	var out []ToolInfo
	for tool, err := range sess.Tools(callCtx, nil) {
		if err != nil {
			m.dropSession(srv.ID)
			return nil, err
		}
		if tool == nil {
			continue
		}
		info := ToolInfo{
			Server:      srv.ID,
			Name:        tool.Name,
			Namespaced:  Namespace(srv.ID, tool.Name),
			Description: tool.Description,
			Mutating:    LooksMutating(tool.Name, tool.Description),
		}
		info.InputSchema = schemaAsMap(tool.InputSchema)
		info.Allowed = CheckCallAllowed(srv, tool.Name, tool.Description) == nil
		out = append(out, info)
	}
	return out, nil
}

func (m *Manager) session(ctx context.Context, srv config.MCPServer) (*mcpsdk.ClientSession, error) {
	m.mu.Lock()
	if sess, ok := m.sessions[srv.ID]; ok && sess != nil {
		m.mu.Unlock()
		return sess, nil
	}
	dial := m.dial
	m.mu.Unlock()

	connectCtx, cancel := context.WithTimeout(ctx, m.connectTimeout)
	defer cancel()
	logging.Info(ctx, "mcp.connect", "server", srv.ID, "transport", srv.Transport, "trace_id", logging.TraceID(ctx))
	sess, err := dial(connectCtx, srv)
	if err != nil {
		logging.Warn(ctx, "mcp.connect failed", "server", srv.ID, "err", err.Error())
		return nil, err
	}
	m.mu.Lock()
	// race: another goroutine may have connected
	if existing, ok := m.sessions[srv.ID]; ok && existing != nil {
		m.mu.Unlock()
		_ = sess.Close()
		return existing, nil
	}
	m.sessions[srv.ID] = sess
	m.mu.Unlock()
	return sess, nil
}

func (m *Manager) dropSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess, ok := m.sessions[id]; ok && sess != nil {
		_ = sess.Close()
	}
	delete(m.sessions, id)
}

func (m *Manager) dialStdio(ctx context.Context, srv config.MCPServer) (*mcpsdk.ClientSession, error) {
	transport := strings.ToLower(strings.TrimSpace(srv.Transport))
	if transport == "" {
		transport = "stdio"
	}
	switch transport {
	case "stdio":
		if strings.TrimSpace(srv.Command) == "" {
			return nil, fmt.Errorf("stdio server %q missing command", srv.ID)
		}
		if err := checkStdioCommand(srv.Command); err != nil {
			return nil, err
		}
		// Do not bind CommandContext to connect timeout — that would kill the
		// long-lived stdio session when the connect deadline expires.
		cmd := exec.Command(srv.Command, srv.Args...)
		cmd.Env = append([]string{}, os.Environ()...)
		for k, v := range srv.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
		// stderr stays on the host process for diagnostics (MCP: don't mix with stdout JSON-RPC)
		cmd.Stderr = os.Stderr
		return m.client.Connect(ctx, &mcpsdk.CommandTransport{Command: cmd}, nil)
	case "sse", "http", "streamable_http":
		return nil, fmt.Errorf("transport %q not implemented yet (stdio only)", transport)
	default:
		return nil, fmt.Errorf("unknown transport %q", transport)
	}
}

// checkStdioCommand fails fast with an actionable message when the binary is missing
// (avoids opaque SDK connect errors for the local echo sample).
func checkStdioCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("stdio command empty")
	}
	// Absolute or relative path: Stat directly.
	if strings.Contains(command, string(os.PathSeparator)) || strings.HasPrefix(command, ".") {
		if _, err := os.Stat(command); err != nil {
			return fmt.Errorf("stdio command not found %q: %w (for sample echo: make mcp-echo && restart make be)", command, err)
		}
		return nil
	}
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("stdio command not on PATH %q: %w", command, err)
	}
	return nil
}

func (m *Manager) server(id string) (config.MCPServer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[id]
	return s, ok
}

func (m *Manager) targetIDs(serverID string) []string {
	serverID = strings.TrimSpace(serverID)
	m.mu.Lock()
	defer m.mu.Unlock()
	if serverID != "" {
		return []string{serverID}
	}
	out := make([]string, 0, len(m.servers))
	for id := range m.servers {
		out = append(out, id)
	}
	return out
}

func flattenContent(content []mcpsdk.Content) ([]map[string]any, string) {
	var blocks []map[string]any
	var texts []string
	for _, c := range content {
		switch t := c.(type) {
		case *mcpsdk.TextContent:
			blocks = append(blocks, map[string]any{"type": "text", "text": t.Text})
			texts = append(texts, t.Text)
		default:
			blocks = append(blocks, map[string]any{"type": fmt.Sprintf("%T", c)})
		}
	}
	return blocks, strings.Join(texts, "\n")
}

// Namespace returns Codex-style mcp__{server}__{tool}.
func Namespace(serverID, tool string) string {
	return "mcp__" + sanitizeSegment(serverID) + "__" + sanitizeSegment(tool)
}

// ParseNamespaced splits mcp__server__tool; ok=false if not namespaced.
func ParseNamespaced(name string) (server, tool string, ok bool) {
	name = strings.TrimSpace(name)
	if !strings.HasPrefix(name, "mcp__") {
		return "", "", false
	}
	rest := strings.TrimPrefix(name, "mcp__")
	i := strings.Index(rest, "__")
	if i <= 0 || i+2 >= len(rest) {
		return "", "", false
	}
	return rest[:i], rest[i+2:], true
}

func sanitizeSegment(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func schemaAsMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	raw, err := jsonMarshal(v)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := jsonUnmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
