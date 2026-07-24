package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/infrastructure/service/docs"
	"buatpostingan/internal/infrastructure/service/tools"
)

func TestListSkillsSeeded(t *testing.T) {
	repoRoot := findRepoRoot(t)
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	skillsRoot := filepath.Join(repoRoot, "resources", "webchat", "skills")
	reg := mustSkillsRegistry(t, toolsRoot, skillsRoot)

	env, err := reg.Execute(context.Background(), service.ToolCall{Name: "list_skills"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("list_skills: %+v", env)
	}
	if env.Meta["data_is_untrusted"] != false {
		t.Fatalf("skills should be trusted project content: %+v", env.Meta)
	}
	if env.Meta["content_trust"] != "project_skill" {
		t.Fatalf("content_trust: %+v", env.Meta)
	}
	data, _ := env.Data.(map[string]any)
	skills, _ := data["skills"].([]map[string]any)
	if len(skills) < 2 {
		t.Fatalf("expected seeded skills, got %#v", data["skills"])
	}
	names := map[string]string{}
	for _, s := range skills {
		n, _ := s["name"].(string)
		d, _ := s["description"].(string)
		names[n] = d
		if d == "" {
			t.Fatalf("empty description for %s", n)
		}
		if strings.Contains(d, "\n---") || strings.HasPrefix(d, "#") {
			t.Fatalf("list_skills must not return full body: %q", d)
		}
	}
	if _, ok := names["writing-post"]; !ok {
		t.Fatalf("missing writing-post: %#v", names)
	}
	if _, ok := names["docs-research"]; !ok {
		t.Fatalf("missing docs-research: %#v", names)
	}
}

func TestReadSkillBody(t *testing.T) {
	repoRoot := findRepoRoot(t)
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	skillsRoot := filepath.Join(repoRoot, "resources", "webchat", "skills")
	reg := mustSkillsRegistry(t, toolsRoot, skillsRoot)

	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "read_skill",
		Arguments: map[string]any{"name": "writing-post"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("read_skill: %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	body, _ := data["body"].(string)
	if !strings.Contains(body, "Writing a post") {
		t.Fatalf("body missing title: %q", body)
	}
	if strings.HasPrefix(strings.TrimSpace(body), "---") {
		t.Fatalf("body should strip frontmatter: %q", body[:min(80, len(body))])
	}
	if data["name"] != "writing-post" {
		t.Fatalf("name=%v", data["name"])
	}
	desc, _ := data["description"].(string)
	if !strings.Contains(desc, "Draft and structure") {
		t.Fatalf("description=%q", desc)
	}
}

func TestReadSkillMissing(t *testing.T) {
	repoRoot := findRepoRoot(t)
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	skillsRoot := filepath.Join(repoRoot, "resources", "webchat", "skills")
	reg := mustSkillsRegistry(t, toolsRoot, skillsRoot)

	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "read_skill",
		Arguments: map[string]any{"name": "no-such-skill"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if env.OK {
		t.Fatalf("expected failure, got %+v", env)
	}
	code, _ := env.Error["code"].(string)
	if code != "skill_not_found" {
		t.Fatalf("code=%s err=%+v", code, env.Error)
	}
}

func TestReadSkillPathEscapeBlocked(t *testing.T) {
	repoRoot := findRepoRoot(t)
	toolsRoot := filepath.Join(repoRoot, "resources", "webchat", "tools")
	skillsRoot := filepath.Join(repoRoot, "resources", "webchat", "skills")
	reg := mustSkillsRegistry(t, toolsRoot, skillsRoot)

	for _, name := range []string{
		"../etc",
		"..",
		"foo/bar",
		"/etc/passwd",
		"writing-post/../../etc",
		".hidden",
		"UPPER",
	} {
		env, err := reg.Execute(context.Background(), service.ToolCall{
			Name:      "read_skill",
			Arguments: map[string]any{"name": name},
		})
		if err != nil {
			t.Fatalf("Execute(%q): %v", name, err)
		}
		if env.OK {
			t.Fatalf("escape %q should fail: %+v", name, env)
		}
		code, _ := env.Error["code"].(string)
		if code != "invalid_skill_name" && code != "path_escape" && code != "skill_not_found" {
			t.Fatalf("name=%q code=%s err=%+v", name, code, env.Error)
		}
	}
}

func TestReadSkillSymlinkEscapeBlocked(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secret, []byte("---\nname: leak\ndescription: no\n---\n# Leak\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	evil := filepath.Join(root, "evil-link")
	if err := os.MkdirAll(evil, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(evil, "SKILL.md")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	reg := mustSkillsRegistry(t, toolsRoot, root)

	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "read_skill",
		Arguments: map[string]any{"name": "evil-link"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if env.OK {
		t.Fatalf("symlink escape should fail: %+v", env)
	}
	code, _ := env.Error["code"].(string)
	if code != "path_escape" {
		t.Fatalf("code=%s err=%+v", code, env.Error)
	}
}

func mustSkillsRegistry(t *testing.T, toolsRoot, skillsRoot string) *tools.Registry {
	t.Helper()
	docsRoot := filepath.Join(findRepoRoot(t), "docs", "webchat")
	storage := t.TempDir()
	idx, err := docs.NewIndex(docsRoot, storage, docs.Options{})
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	reg, err := tools.NewRegistry(toolsRoot, idx, tools.Options{SkillsRoot: skillsRoot})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return reg
}
