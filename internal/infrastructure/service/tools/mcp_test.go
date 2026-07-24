package tools

import (
	"context"
	"strings"
	"testing"

	"buatpostingan/internal/domain/service"
	mcpmgr "buatpostingan/internal/infrastructure/service/mcp"
)

type fakeMCP struct {
	enabled  bool
	servers  []string
	tools    []mcpmgr.ToolInfo
	errs     map[string]string
	called   bool
	lastSrv  string
	lastTool string
	lastArgs map[string]any
	result   mcpmgr.CallResult
	callErr  error
}

func (f *fakeMCP) Enabled() bool { return f.enabled }
func (f *fakeMCP) ServerIDs() []string {
	if f.servers == nil {
		return nil
	}
	return append([]string(nil), f.servers...)
}
func (f *fakeMCP) ListTools(_ context.Context, _ string) ([]mcpmgr.ToolInfo, map[string]string) {
	return f.tools, f.errs
}
func (f *fakeMCP) CallTool(_ context.Context, serverID, toolName string, args map[string]any) (mcpmgr.CallResult, error) {
	f.called = true
	f.lastSrv = serverID
	f.lastTool = toolName
	f.lastArgs = args
	return f.result, f.callErr
}

func TestListMCPToolsDisabled(t *testing.T) {
	reg := &Registry{
		mcp:     &fakeMCP{enabled: false},
		schemas: map[string]map[string]any{"list_mcp_tools": {"name": "list_mcp_tools"}},
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{Name: "list_mcp_tools"})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	data, _ := env.Data.(map[string]any)
	if data["mcp_enabled"] != false {
		t.Fatalf("%+v", data)
	}
	if hint, _ := data["hint"].(string); hint == "" {
		t.Fatalf("expected hint when disabled: %+v", data)
	}
}

func TestListMCPToolsNoServersHint(t *testing.T) {
	reg := &Registry{
		mcp:     &fakeMCP{enabled: true, servers: nil, errs: map[string]string{}},
		schemas: map[string]map[string]any{"list_mcp_tools": {"name": "list_mcp_tools"}},
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{Name: "list_mcp_tools"})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := env.Data.(map[string]any)
	if data["mcp_enabled"] != true {
		t.Fatalf("%+v", data)
	}
	if data["servers_configured"] != 0 {
		t.Fatalf("%+v", data)
	}
	hint, _ := data["hint"].(string)
	if hint == "" || !strings.Contains(hint, "config.example.json") {
		t.Fatalf("expected config.example hint, got %q", hint)
	}
}

func TestCallMCPToolNamespaced(t *testing.T) {
	fake := &fakeMCP{
		enabled: true,
		result: mcpmgr.CallResult{
			Server:  "echo",
			Tool:    "echo",
			RawText: "hi",
			Trusted: true,
			Content: []map[string]any{{"type": "text", "text": "hi"}},
		},
	}
	reg := &Registry{
		mcp:     fake,
		schemas: map[string]map[string]any{"call_mcp_tool": {"name": "call_mcp_tool"}},
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name: "call_mcp_tool",
		Arguments: map[string]any{
			"name":      "mcp__echo__echo",
			"arguments": map[string]any{"message": "hi"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK {
		t.Fatalf("%+v", env)
	}
	if !fake.called || fake.lastSrv != "echo" || fake.lastTool != "echo" {
		t.Fatalf("call=%v srv=%q tool=%q", fake.called, fake.lastSrv, fake.lastTool)
	}
	if env.Meta["data_is_untrusted"] != false {
		t.Fatalf("trusted meta: %+v", env.Meta)
	}
	if env.Meta["content_trust"] != "project_mcp" {
		t.Fatalf("content_trust: %+v", env.Meta)
	}
}

func TestCallMCPToolEmptyArgsReturnsCatalogExample(t *testing.T) {
	fake := &fakeMCP{
		enabled: true,
		tools: []mcpmgr.ToolInfo{{
			Server:  "echo",
			Name:    "echo",
			Allowed: true,
			InputSchema: map[string]any{
				"type":       "object",
				"required":   []any{"message"},
				"properties": map[string]any{"message": map[string]any{"type": "string"}},
			},
		}},
	}
	reg := &Registry{
		mcp:     fake,
		schemas: map[string]map[string]any{"call_mcp_tool": {"name": "call_mcp_tool"}},
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "call_mcp_tool",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if env.OK || fake.called {
		t.Fatalf("expected validation before execution: env=%+v called=%v", env, fake.called)
	}
	message, _ := env.Error["message"].(string)
	for _, want := range []string{`"server":"<server>"`, `"tool":"<tool>"`, `"server":"echo"`, `"message":"required-value"`} {
		if !strings.Contains(message, want) {
			t.Fatalf("error missing %q: %s", want, message)
		}
	}
}

func TestCallMCPToolServerToolArguments(t *testing.T) {
	fake := &fakeMCP{
		enabled: true,
		result: mcpmgr.CallResult{
			Server:  "echo",
			Tool:    "echo",
			RawText: "hello",
		},
	}
	reg := &Registry{
		mcp:     fake,
		schemas: map[string]map[string]any{"call_mcp_tool": {"name": "call_mcp_tool"}},
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name: "call_mcp_tool",
		Arguments: map[string]any{
			"server":    "echo",
			"tool":      "echo",
			"arguments": map[string]any{"message": "hello"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !env.OK || !fake.called {
		t.Fatalf("expected execution: env=%+v called=%v", env, fake.called)
	}
	if fake.lastArgs["message"] != "hello" {
		t.Fatalf("arguments were not forwarded: %+v", fake.lastArgs)
	}
}

func TestCallMCPToolMutationError(t *testing.T) {
	fake := &fakeMCP{enabled: true, callErr: errString("mutation_denied")}
	reg := &Registry{
		mcp:     fake,
		schemas: map[string]map[string]any{"call_mcp_tool": {"name": "call_mcp_tool"}},
	}
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name: "call_mcp_tool",
		Arguments: map[string]any{
			"server": "echo",
			"tool":   "delete_file",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if env.OK {
		t.Fatal("expected failure")
	}
	if env.Error["code"] != "mutation_denied" {
		t.Fatalf("%+v", env.Error)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
