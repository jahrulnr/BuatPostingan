package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyVars(t *testing.T) {
	got := applyVars("hi {{admin_display_name}} / {{locale}}", map[string]string{
		"admin_display_name": "Ada",
		"locale":             "id",
	})
	if got != "hi Ada / id" {
		t.Fatalf("got %q", got)
	}
}

func TestInjectPrompts(t *testing.T) {
	root := t.TempDir()
	// system.md is shipped static — vars left untouched.
	if err := os.WriteFile(filepath.Join(root, "system.md"), []byte("static role text {{admin_display_name}} stays literal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs.md"), []byte("docs workflow {{admin_display_name}} stays literal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills.md"), []byte("skills workflow {{admin_display_name}} stays literal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pages.md"), []byte("page rules {{admin_display_name}} stay literal"), 0o644); err != nil {
		t.Fatal(err)
	}
	// developer.md is the single injection surface.
	if err := os.WriteFile(filepath.Join(root, "developer.md"), []byte("dev uid={{admin_user_id}} tools={{available_tools}} docs={{indexed_document_count}} name={{admin_display_name}} path={{ui_path}}"), 0o644); err != nil {
		t.Fatal(err)
	}

	msgs, err := injectPrompts(root, []map[string]any{
		{"role": "system", "content": "drop me"},
		{"role": "developer", "content": "drop me too"},
		{"role": "user", "content": "hello"},
	}, promptVars{
		AdminDisplayName:     "Ada",
		AdminUserID:          42,
		AvailableTools:       "",
		IndexedDocumentCount: 3,
		UIPath:               "http://localhost:5173/#/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 6 {
		t.Fatalf("len=%d %#v", len(msgs), msgs)
	}
	sys, _ := msgs[0]["content"].(string)
	if strings.Contains(sys, "Ada") || !strings.Contains(sys, "{{admin_display_name}}") {
		t.Fatalf("system.md must stay static, got=%q", sys)
	}
	docs, _ := msgs[1]["content"].(string)
	if docs != "docs workflow {{admin_display_name}} stays literal" {
		t.Fatalf("docs.md must be static and follow system.md, got=%q", docs)
	}
	skills, _ := msgs[2]["content"].(string)
	if skills != "skills workflow {{admin_display_name}} stays literal" {
		t.Fatalf("skills.md must be static and follow docs.md, got=%q", skills)
	}
	pages, _ := msgs[3]["content"].(string)
	if pages != "page rules {{admin_display_name}} stay literal" {
		t.Fatalf("pages.md must be static and follow skills.md, got=%q", pages)
	}
	dev, _ := msgs[4]["content"].(string)
	if !strings.Contains(dev, "uid=42") || !strings.Contains(dev, "tools=(none)") || !strings.Contains(dev, "docs=3") || !strings.Contains(dev, "name=Ada") || !strings.Contains(dev, "path=http://localhost:5173/#/") {
		t.Fatalf("developer=%q", dev)
	}
	if msgs[5]["role"] != "user" || msgs[5]["content"] != "hello" {
		t.Fatalf("user=%#v", msgs[5])
	}

	msgs, err = injectPrompts(root, []map[string]any{
		{"role": "system", "content": "Conversation summary:\nprior context"},
		{"role": "user", "content": "latest"},
	}, promptVars{AdminDisplayName: "Ada", AvailableTools: "docs_search"})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 7 {
		t.Fatalf("want sys+docs+skills+pages+dev+summary+user got %d %#v", len(msgs), msgs)
	}
	sum, _ := msgs[5]["content"].(string)
	if !strings.HasPrefix(sum, "Conversation summary:") {
		t.Fatalf("summary stripped: %#v", msgs)
	}
}

func TestInjectPromptsMissingFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"system.md", "docs.md", "skills.md", "pages.md", "developer.md"} {
		_, err := injectPrompts(root, nil, promptVars{})
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("missing %s: err=%v", name, err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
