package mcp

import (
	"fmt"
	"strings"

	"buatpostingan/internal/config"
)

// mutationTokens are substrings that strongly suggest a write/side-effect tool.
var mutationTokens = []string{
	"write", "create", "update", "delete", "remove", "destroy", "drop",
	"truncate", "insert", "upsert", "mutate", "publish", "upload", "send",
	"post_", "put_", "patch_", "exec", "execute", "shell", "bash", "cmd",
	"run_command", "run_script", "kill", "restart", "apply", "set_",
	"chmod", "chown", "mv_", "move", "rename", "unlink",
}

// LooksMutating reports whether a tool name/description looks mutating.
func LooksMutating(name, description string) bool {
	blob := strings.ToLower(strings.TrimSpace(name) + " " + strings.TrimSpace(description))
	if blob == "" {
		return false
	}
	for _, tok := range mutationTokens {
		if strings.Contains(blob, tok) {
			return true
		}
	}
	return false
}

// CheckCallAllowed enforces deny/allow lists and default-deny mutations.
func CheckCallAllowed(srv config.MCPServer, toolName, description string) error {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return fmt.Errorf("tool_required")
	}
	for _, d := range srv.DenyTools {
		if strings.EqualFold(strings.TrimSpace(d), toolName) {
			return fmt.Errorf("tool_denied")
		}
	}
	if len(srv.AllowTools) > 0 {
		ok := false
		for _, a := range srv.AllowTools {
			if strings.EqualFold(strings.TrimSpace(a), toolName) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("tool_not_allowed")
		}
	}
	mutating := LooksMutating(toolName, description)
	if mutating && !srv.AllowMutations {
		return fmt.Errorf("mutation_denied")
	}
	if mutating && srv.AllowMutations && len(srv.AllowTools) > 0 {
		// allow_mutations still requires an explicit allow_tools hit (already checked).
		return nil
	}
	if mutating && srv.AllowMutations && len(srv.AllowTools) == 0 {
		// Opt-in mutations without an allowlist is too broad for the reader lock.
		return fmt.Errorf("mutation_denied")
	}
	return nil
}
