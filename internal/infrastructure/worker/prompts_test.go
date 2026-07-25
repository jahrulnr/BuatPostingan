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
	// developer.md is the single injection surface.
	if err := os.WriteFile(filepath.Join(root, "developer.md"), []byte("dev uid={{admin_user_id}} tools={{available_tools}} docs={{indexed_document_count}} name={{admin_display_name}}"), 0o644); err != nil {
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
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("len=%d %#v", len(msgs), msgs)
	}
	sys, _ := msgs[0]["content"].(string)
	if strings.Contains(sys, "Ada") || !strings.Contains(sys, "{{admin_display_name}}") {
		t.Fatalf("system.md must stay static, got=%q", sys)
	}
	dev, _ := msgs[1]["content"].(string)
	if !strings.Contains(dev, "uid=42") || !strings.Contains(dev, "tools=(none)") || !strings.Contains(dev, "docs=3") || !strings.Contains(dev, "name=Ada") {
		t.Fatalf("developer=%q", dev)
	}
	if msgs[2]["role"] != "user" || msgs[2]["content"] != "hello" {
		t.Fatalf("user=%#v", msgs[2])
	}

	msgs, err = injectPrompts(root, []map[string]any{
		{"role": "system", "content": "Conversation summary:\nprior context"},
		{"role": "user", "content": "latest"},
	}, promptVars{AdminDisplayName: "Ada", AvailableTools: "search_docs"})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 4 {
		t.Fatalf("want sys+dev+summary+user got %d %#v", len(msgs), msgs)
	}
	sum, _ := msgs[2]["content"].(string)
	if !strings.HasPrefix(sum, "Conversation summary:") {
		t.Fatalf("summary stripped: %#v", msgs)
	}
}

func TestInjectPromptsMissingFiles(t *testing.T) {
	root := t.TempDir()
	_, err := injectPrompts(root, nil, promptVars{})
	if err == nil || !strings.Contains(err.Error(), "missing prompt") {
		t.Fatalf("err=%v", err)
	}
	_ = os.WriteFile(filepath.Join(root, "system.md"), []byte("x"), 0o644)
	_, err = injectPrompts(root, nil, promptVars{})
	if err == nil || !strings.Contains(err.Error(), "developer.md") {
		t.Fatalf("err=%v", err)
	}
}
