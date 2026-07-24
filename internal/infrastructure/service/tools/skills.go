package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"buatpostingan/internal/domain/service"
)

var skillNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// skillsFS lists and reads SKILL.md files under a jailed skills root.
type skillsFS struct {
	root string // absolute, EvalSymlinks when possible; empty = unavailable
}

func newSkillsFS(skillsRoot string) (*skillsFS, error) {
	skillsRoot = strings.TrimSpace(skillsRoot)
	if skillsRoot == "" {
		return &skillsFS{root: ""}, nil
	}
	abs, err := filepath.Abs(skillsRoot)
	if err != nil {
		return nil, fmt.Errorf("skills root unavailable: %w", err)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if st, stErr := os.Stat(abs); stErr != nil || !st.IsDir() {
			return &skillsFS{root: ""}, nil // soft: missing root → empty catalog
		}
		real = abs
	}
	st, err := os.Stat(real)
	if err != nil || !st.IsDir() {
		return &skillsFS{root: ""}, nil
	}
	return &skillsFS{root: real}, nil
}

func (r *Registry) execListSkills(_ map[string]any) service.ToolEnvelope {
	started := time.Now()
	if r.skills == nil || r.skills.root == "" {
		return service.ToolEnvelope{
			OK:   true,
			Tool: "list_skills",
			Data: map[string]any{"skills": []map[string]any{}},
			Meta: skillMeta(false, 0, started),
		}
	}
	entries, err := os.ReadDir(r.skills.root)
	if err != nil {
		return skillFail("list_skills", "tool_error", "skills root could not be read", started)
	}
	var skills []map[string]any
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !skillNameRe.MatchString(name) {
			continue
		}
		meta, body, err := r.skills.readSkillFile(name)
		if err != nil {
			continue
		}
		_ = body
		desc := meta["description"]
		if desc == "" {
			desc = name
		}
		skills = append(skills, map[string]any{
			"name":        coalesceSkillName(meta["name"], name),
			"description": desc,
		})
	}
	sort.Slice(skills, func(i, j int) bool {
		ni, _ := skills[i]["name"].(string)
		nj, _ := skills[j]["name"].(string)
		return ni < nj
	})
	return service.ToolEnvelope{
		OK:   true,
		Tool: "list_skills",
		Data: map[string]any{"skills": skills},
		Meta: skillMeta(false, len(skills), started),
	}
}

func (r *Registry) execReadSkill(args map[string]any) service.ToolEnvelope {
	started := time.Now()
	name := strings.TrimSpace(asString(args["name"]))
	if name == "" {
		return skillFail("read_skill", "validation", "name required", started)
	}
	if !skillNameRe.MatchString(name) {
		return skillFail("read_skill", "invalid_skill_name", "name must be kebab-case ([a-z0-9-], max 64)", started)
	}
	if r.skills == nil || r.skills.root == "" {
		return skillFail("read_skill", "skills_unavailable", "skills root not configured or missing", started)
	}
	meta, body, err := r.skills.readSkillFile(name)
	if err != nil {
		code := "skill_not_found"
		msg := err.Error()
		if strings.Contains(msg, "path escape") || strings.Contains(msg, "outside skills root") {
			code = "path_escape"
		}
		return skillFail("read_skill", code, msg, started)
	}
	skillName := coalesceSkillName(meta["name"], name)
	return service.ToolEnvelope{
		OK:   true,
		Tool: "read_skill",
		Data: map[string]any{
			"name":        skillName,
			"description": meta["description"],
			"body":        body,
		},
		Meta: skillMeta(false, 1, started),
	}
}

func (s *skillsFS) readSkillFile(name string) (map[string]string, string, error) {
	if s == nil || s.root == "" {
		return nil, "", fmt.Errorf("skills unavailable")
	}
	if !skillNameRe.MatchString(name) {
		return nil, "", fmt.Errorf("invalid skill name")
	}
	// Jail: only <root>/<name>/SKILL.md
	candidate := filepath.Join(s.root, name, "SKILL.md")
	real, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if _, stErr := os.Stat(candidate); stErr != nil {
			return nil, "", fmt.Errorf("skill not found: %s", name)
		}
		real = filepath.Clean(candidate)
	}
	if !underRoot(s.root, real) {
		return nil, "", fmt.Errorf("path escape: outside skills root")
	}
	raw, err := os.ReadFile(real)
	if err != nil {
		return nil, "", fmt.Errorf("skill not found: %s", name)
	}
	meta, body := parseSkillMarkdown(string(raw))
	return meta, body, nil
}

func underRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func parseSkillMarkdown(raw string) (map[string]string, string) {
	meta := map[string]string{}
	body := raw
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "---") {
		return meta, strings.TrimSpace(body)
	}
	rest := strings.TrimPrefix(trimmed, "---")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return meta, strings.TrimSpace(body)
	}
	fm := rest[:end]
	body = strings.TrimSpace(rest[end+len("\n---"):])
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "description" && isYAMLFoldIndicator(val) {
			// folded block: collected by parseFoldedDescription
			continue
		}
		if (strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)) ||
			(strings.HasPrefix(val, `'`) && strings.HasSuffix(val, `'`)) {
			val = val[1 : len(val)-1]
		}
		if key == "name" || key == "description" {
			meta[key] = val
		}
	}
	// Folded description (>-style): lines after "description: >" until next key or end of fm
	if meta["description"] == "" {
		meta["description"] = parseFoldedDescription(fm)
	}
	return meta, body
}

func parseFoldedDescription(fm string) string {
	lines := strings.Split(fm, "\n")
	var collecting bool
	var parts []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if !collecting {
			if strings.HasPrefix(trim, "description:") {
				rest := strings.TrimSpace(strings.TrimPrefix(trim, "description:"))
				if rest == "" || isYAMLFoldIndicator(rest) {
					collecting = true
					continue
				}
				return strings.Trim(rest, `"'`)
			}
			continue
		}
		if trim == "" {
			if len(parts) > 0 {
				parts = append(parts, "")
			}
			continue
		}
		// next top-level key ends the block
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(trim, ":") {
			key := strings.TrimSpace(strings.SplitN(trim, ":", 2)[0])
			if key != "" && !strings.Contains(key, " ") {
				break
			}
		}
		parts = append(parts, strings.TrimSpace(line))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func isYAMLFoldIndicator(v string) bool {
	switch v {
	case ">", "|", ">-", ">|", "|-", "|+":
		return true
	default:
		return false
	}
}

func coalesceSkillName(fromMeta, dirName string) string {
	if n := strings.TrimSpace(fromMeta); n != "" && skillNameRe.MatchString(n) {
		return n
	}
	return dirName
}

func skillMeta(truncated bool, count int, started time.Time) map[string]any {
	return map[string]any{
		"truncated":         truncated,
		"count":             count,
		"took_ms":           int(time.Since(started).Milliseconds()),
		"data_is_untrusted": false,
		"content_trust":     "project_skill",
	}
}

func skillFail(tool, code, message string, started time.Time) service.ToolEnvelope {
	return service.ToolEnvelope{
		OK:   false,
		Tool: tool,
		Data: nil,
		Error: map[string]any{
			"code":    code,
			"message": message,
		},
		Meta: skillMeta(false, 0, started),
	}
}
