package tools

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"buatpostingan/internal/domain/service"
	mcpmgr "buatpostingan/internal/infrastructure/service/mcp"
)

func (r *Registry) execListMCPTools(ctx context.Context, args map[string]any) service.ToolEnvelope {
	started := time.Now()
	if r.mcp == nil || !r.mcp.Enabled() {
		return service.ToolEnvelope{
			OK:   true,
			Tool: "list_mcp_tools",
			Data: map[string]any{
				"tools":              []map[string]any{},
				"server_errors":      map[string]string{},
				"mcp_enabled":        false,
				"servers_configured": 0,
				"hint":               "MCP is disabled (BP_MCP_ENABLED=false or mcp.enabled=false). Enable it and add mcp.servers in storage/config.json.",
			},
			Meta: mcpMeta(false, 0, started, false),
		}
	}
	serverID := strings.TrimSpace(asString(args["server"]))
	configured := r.mcp.ServerIDs()
	listed, serverErrors := r.mcp.ListTools(ctx, serverID)
	sort.Slice(listed, func(i, j int) bool {
		if listed[i].Server != listed[j].Server {
			return listed[i].Server < listed[j].Server
		}
		return listed[i].Name < listed[j].Name
	})
	tools := make([]map[string]any, 0, len(listed))
	for _, t := range listed {
		row := map[string]any{
			"server":      t.Server,
			"name":        t.Name,
			"namespaced":  t.Namespaced,
			"description": t.Description,
			"mutating":    t.Mutating,
			"allowed":     t.Allowed,
		}
		if t.InputSchema != nil {
			row["input_schema"] = t.InputSchema
		}
		tools = append(tools, row)
	}
	if serverErrors == nil {
		serverErrors = map[string]string{}
	}
	data := map[string]any{
		"tools":              tools,
		"server_errors":      serverErrors,
		"mcp_enabled":        true,
		"servers_configured": len(configured),
	}
	// Empty catalog with empty server_errors usually means zero servers in
	// config.json (BP_MCP_ENABLED defaults true). Surface that explicitly so
	// the model/operator do not treat it as a silent connect success.
	if len(configured) == 0 {
		data["hint"] = "No MCP servers configured in storage/config.json (mcp.servers is empty/missing). Run `make mcp-echo`, then restart `make be`."
	} else if len(tools) == 0 && len(serverErrors) == 0 && serverID != "" {
		data["hint"] = "Server filter matched a configured id but returned no tools."
	} else if len(tools) == 0 && len(serverErrors) > 0 {
		data["hint"] = "MCP servers are configured but listing failed for one or more (see server_errors). For the sample echo server, run `make mcp-echo` and ensure command paths are relative to the process cwd (repo root)."
	}
	return service.ToolEnvelope{
		OK:   true,
		Tool: "list_mcp_tools",
		Data: data,
		Meta: mcpMeta(false, len(tools), started, false),
	}
}

func (r *Registry) execCallMCPTool(ctx context.Context, args map[string]any) service.ToolEnvelope {
	started := time.Now()
	if r.mcp == nil || !r.mcp.Enabled() {
		return mcpFail("call_mcp_tool", "mcp_disabled", "MCP is disabled or not configured", started, false)
	}
	serverID := strings.TrimSpace(asString(args["server"]))
	toolName := strings.TrimSpace(asString(args["tool"]))
	if ns := strings.TrimSpace(asString(args["name"])); ns != "" {
		if s, t, ok := mcpmgr.ParseNamespaced(ns); ok {
			if serverID == "" {
				serverID = s
			}
			if toolName == "" {
				toolName = t
			}
		} else if toolName == "" {
			toolName = ns
		}
	}
	if serverID == "" {
		return mcpFail("call_mcp_tool", "validation", r.mcpCallUsage(ctx), started, false)
	}
	if toolName == "" {
		return mcpFail("call_mcp_tool", "validation", r.mcpCallUsage(ctx), started, false)
	}
	callArgs, _ := args["arguments"].(map[string]any)
	if callArgs == nil {
		callArgs = map[string]any{}
	}
	res, err := r.mcp.CallTool(ctx, serverID, toolName, callArgs)
	if err != nil {
		code := "mcp_call_failed"
		msg := err.Error()
		switch msg {
		case "mcp_disabled", "server_not_found", "server_disabled",
			"tool_denied", "tool_not_allowed", "mutation_denied", "tool_required":
			code = msg
		}
		return mcpFail("call_mcp_tool", code, msg, started, false)
	}
	untrusted := !res.Trusted
	meta := mcpMeta(false, 1, started, untrusted)
	if res.Trusted {
		meta["content_trust"] = "project_mcp"
	}
	ok := !res.IsError
	env := service.ToolEnvelope{
		OK:   ok,
		Tool: "call_mcp_tool",
		Data: map[string]any{
			"server":  res.Server,
			"tool":    res.Tool,
			"content": res.Content,
			"text":    res.RawText,
		},
		Meta: meta,
	}
	if res.IsError {
		env.Error = map[string]any{
			"code":    "mcp_tool_error",
			"message": "MCP tool returned isError=true",
		}
	}
	return env
}

func (r *Registry) mcpCallUsage(ctx context.Context) string {
	message := `call_mcp_tool requires {"server":"<server>","tool":"<tool>","arguments":{...}} or {"name":"mcp__<server>__<tool>","arguments":{...}}; copy identifiers and input requirements from list_mcp_tools`
	listed, _ := r.mcp.ListTools(ctx, "")
	sort.Slice(listed, func(i, j int) bool {
		if listed[i].Server != listed[j].Server {
			return listed[i].Server < listed[j].Server
		}
		return listed[i].Name < listed[j].Name
	})
	for _, tool := range listed {
		if !tool.Allowed {
			continue
		}
		example := map[string]any{
			"server":    tool.Server,
			"tool":      tool.Name,
			"arguments": requiredMCPArguments(tool.InputSchema),
		}
		if raw, err := json.Marshal(example); err == nil {
			return message + "; catalog example: " + string(raw)
		}
		break
	}
	return message + `; example: {"server":"echo","tool":"echo","arguments":{"message":"hello"}}`
}

func requiredMCPArguments(schema map[string]any) map[string]any {
	arguments := map[string]any{}
	required, _ := schema["required"].([]any)
	properties, _ := schema["properties"].(map[string]any)
	for _, rawName := range required {
		name, _ := rawName.(string)
		if name == "" {
			continue
		}
		property, _ := properties[name].(map[string]any)
		switch property["type"] {
		case "boolean":
			arguments[name] = false
		case "integer", "number":
			arguments[name] = 0
		case "array":
			arguments[name] = []any{}
		case "object":
			arguments[name] = map[string]any{}
		default:
			arguments[name] = "required-value"
		}
	}
	return arguments
}

func mcpMeta(truncated bool, count int, started time.Time, untrusted bool) map[string]any {
	return map[string]any{
		"truncated":         truncated,
		"count":             count,
		"took_ms":           int(time.Since(started).Milliseconds()),
		"data_is_untrusted": untrusted,
	}
}

func mcpFail(tool, code, message string, started time.Time, untrusted bool) service.ToolEnvelope {
	return service.ToolEnvelope{
		OK:   false,
		Tool: tool,
		Data: nil,
		Error: map[string]any{
			"code":    code,
			"message": message,
		},
		Meta: mcpMeta(false, 0, started, untrusted),
	}
}
