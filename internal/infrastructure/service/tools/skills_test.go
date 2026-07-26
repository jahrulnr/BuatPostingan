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
	if _, ok := names["article-writing"]; !ok {
		t.Fatalf("missing article-writing: %#v", names)
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
		Arguments: map[string]any{"name": "article-writing"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("read_skill: %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	body, _ := data["body"].(string)
	if !strings.Contains(body, "Article Writing") {
		t.Fatalf("body missing title: %q", body)
	}
	if strings.HasPrefix(strings.TrimSpace(body), "---") {
		t.Fatalf("body should strip frontmatter: %q", body[:min(80, len(body))])
	}
	if data["name"] != "article-writing" {
		t.Fatalf("name=%v", data["name"])
	}
	desc, _ := data["description"].(string)
	if !strings.Contains(desc, "long-form content") {
		t.Fatalf("description=%q", desc)
	}
	example := filepath.Join(skillsRoot, "article-writing", "examples", "tutorial-howto.md")
	if !strings.Contains(body, "- "+example) {
		t.Fatalf("body missing absolute supporting file %q: %q", example, body)
	}
	if strings.Contains(body, filepath.Join(skillsRoot, "article-writing", "SKILL.md")) {
		t.Fatalf("body must not list its own SKILL.md: %q", body)
	}
}

func TestReadSkillAppendsOnlyJailedRegularSupportingFiles(t *testing.T) {
	toolsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "tools")
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo")
	if err := os.MkdirAll(filepath.Join(skillDir, "examples", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"SKILL.md":                    "---\nname: demo\ndescription: Demo skill\n---\n# Demo\n",
		"references.md":               "reference",
		"examples/z-last.md":          "last",
		"examples/nested/a-first.md":  "first",
		"scripts/validate-example.js": "export default true",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(skillDir, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside-secret.md")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(skillDir, "examples", "outside-link.md")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	reg := mustSkillsRegistry(t, toolsRoot, root)
	env, err := reg.Execute(context.Background(), service.ToolCall{
		Name:      "read_skill",
		Arguments: map[string]any{"name": "demo"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !env.OK {
		t.Fatalf("read_skill: %+v", env)
	}
	data, _ := env.Data.(map[string]any)
	body, _ := data["body"].(string)
	wantPaths := []string{
		filepath.Join(skillDir, "examples", "nested", "a-first.md"),
		filepath.Join(skillDir, "examples", "z-last.md"),
		filepath.Join(skillDir, "references.md"),
		filepath.Join(skillDir, "scripts", "validate-example.js"),
	}
	if !strings.Contains(body, "\n---\nAdditional files (read with `read_file` when needed):\n") {
		t.Fatalf("missing supporting-files footer: %q", body)
	}
	last := -1
	for _, want := range wantPaths {
		if !filepath.IsAbs(want) {
			t.Fatalf("test path must be absolute: %q", want)
		}
		at := strings.Index(body, "- "+want)
		if at < 0 {
			t.Fatalf("missing supporting file %q: %q", want, body)
		}
		if at <= last {
			t.Fatalf("supporting files are not sorted: %q", body)
		}
		last = at
	}
	if strings.Contains(body, outside) || strings.Contains(body, "outside-link.md") {
		t.Fatalf("symlinked file must not be exposed: %q", body)
	}
	if strings.Contains(body, filepath.Join(skillDir, "SKILL.md")) {
		t.Fatalf("SKILL.md must not list itself: %q", body)
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
	docsRoot := filepath.Join(findRepoRoot(t), "resources", "webchat", "docs")
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
