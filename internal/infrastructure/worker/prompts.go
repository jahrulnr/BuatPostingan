package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type promptVars struct {
	AdminDisplayName     string
	AdminUserID          int64
	AvailableTools       string
	IndexedDocumentCount int
	Workspace            string
}

func injectPrompts(promptsRoot string, messages []map[string]any, vars promptVars) ([]map[string]any, error) {
	systemPath := filepath.Join(promptsRoot, "system.md")
	developerPath := filepath.Join(promptsRoot, "developer.md")
	systemRaw, err := os.ReadFile(systemPath)
	if err != nil {
		return nil, fmt.Errorf("missing prompt %s: %w", systemPath, err)
	}
	developerRaw, err := os.ReadFile(developerPath)
	if err != nil {
		return nil, fmt.Errorf("missing prompt %s: %w", developerPath, err)
	}

	cwd, _ := os.Getwd()
	if cwd == "" {
		cwd = "."
	}
	if strings.TrimSpace(vars.Workspace) != "" {
		cwd = strings.TrimSpace(vars.Workspace)
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = cwd
	}

	availableTools := vars.AvailableTools
	if availableTools == "" {
		availableTools = "(none)"
	}
	repl := map[string]string{
		"admin_display_name":     vars.AdminDisplayName,
		"admin_user_id":          strconv.FormatInt(vars.AdminUserID, 10),
		"admin_role_name":        "admin",
		"admin_role_id":          "0",
		"locale":                 "id",
		"cms_environment":        "local",
		"available_tools":        availableTools,
		"indexed_document_count": strconv.Itoa(vars.IndexedDocumentCount),
		"pii_redaction":          "false",
		"current_admin_path":     "",
		"last_entity_ref":        "",
		"conversation_goal":      "",
		"cwd":                    cwd,
		"home":                   home,
		"current_date":           time.Now().Format("2006-01-02"),
	}

	// system.md is shipped as static text (no {{var}} placeholders); used verbatim.
	system := string(systemRaw)
	// developer.md is the single injection surface for runtime context.
	developer := applyVars(string(developerRaw), repl)

	out := []map[string]any{
		{"role": "system", "content": system},
		{"role": "system", "content": developer},
	}
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		if role == "system" || role == "developer" {
			content, _ := msg["content"].(string)
			if strings.HasPrefix(content, "Conversation summary:") {
				out = append(out, msg)
			}
			continue
		}
		out = append(out, msg)
	}
	return out, nil
}

func applyVars(text string, vars map[string]string) string {
	for k, v := range vars {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	return text
}
