package llm

import (
	"strings"
	"testing"

	"buatpostingan/internal/domain/service"
)

func TestExtractXMLToolCalls_singleWriteFile(t *testing.T) {
	text := `Some intro.
<tool_call>
<function=write_file>
<parameter=append>False</parameter>
<parameter=path>/home/user/profile.html</parameter>
<parameter=content><!DOCTYPE html>
<html lang="id">
<head><title>Profile</title></head>
<body>Hello</body>
</html></parameter>
</function>
</tool_call>
Some outro.`

	calls, stripped := extractXMLToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "write_file" {
		t.Fatalf("expected write_file, got %s", calls[0].Name)
	}
	if calls[0].Arguments["path"] != "/home/user/profile.html" {
		t.Fatalf("unexpected path: %v", calls[0].Arguments["path"])
	}
	if calls[0].Arguments["append"] != false {
		t.Fatalf("expected append=false, got %v", calls[0].Arguments["append"])
	}
	content, _ := calls[0].Arguments["content"].(string)
	if !strings.Contains(content, "<!DOCTYPE html>") {
		t.Fatalf("content missing doctype: %q", content)
	}
	if strings.Contains(stripped, "<tool_call>") {
		t.Fatalf("stripped text still contains tool call xml: %q", stripped)
	}
	if !strings.Contains(stripped, "Some intro.") || !strings.Contains(stripped, "Some outro.") {
		t.Fatalf("stripped text lost surrounding text: %q", stripped)
	}
}

func TestExtractXMLToolCalls_removesNestedParameterFromContent(t *testing.T) {
	text := `<tool_call><function=write_file>
<parameter=path>/tmp/x.html</parameter>
<parameter=content><html><body>Hi</body></html><parameter=append>False</parameter></parameter>
</function></tool_call>`

	calls, _ := extractXMLToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Arguments["append"] != false {
		t.Fatalf("expected append=false, got %#v", calls[0].Arguments["append"])
	}
	content, _ := calls[0].Arguments["content"].(string)
	if strings.Contains(content, "parameter=append") || !strings.Contains(content, "<body>Hi</body>") {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestExtractXMLToolCalls_contentContainsClosingParameter(t *testing.T) {
	// Ensure a literal </parameter> inside the content value does not trick the parser.
	text := `<tool_call><function=write_file>
<parameter=path>/tmp/x.html</parameter>
<parameter=content>before </parameter> after</parameter>
</function></tool_call>`

	calls, _ := extractXMLToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	content, _ := calls[0].Arguments["content"].(string)
	if content != "before </parameter> after" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestRecoverXMLToolCalls_usesStreamWhenPayloadHasEmptyNativeCall(t *testing.T) {
	result := service.LLMResult{ToolCalls: []service.ToolCall{{
		CallID: "call_api", Name: "write_file", Arguments: map[string]any{},
	}}}
	streamed := `<tool_call><function=write_file>
<parameter=path>/tmp/a.html</parameter>
<parameter=content>hi</parameter>
</function></tool_call>`

	recovered := RecoverXMLToolCalls(result, streamed)
	if len(recovered.ToolCalls) != 1 {
		t.Fatalf("expected 1 recovered call, got %#v", recovered.ToolCalls)
	}
	if recovered.ToolCalls[0].Arguments["path"] != "/tmp/a.html" {
		t.Fatalf("unexpected args: %#v", recovered.ToolCalls[0].Arguments)
	}
	if recovered.ToolCalls[0].Arguments["content"] != "hi" {
		t.Fatalf("unexpected args: %#v", recovered.ToolCalls[0].Arguments)
	}
}

func TestMergeXMLToolCalls_fillsEmptyAPIArgs(t *testing.T) {
	api := []service.ToolCall{
		{CallID: "call_api", Name: "write_file", Arguments: map[string]any{}},
	}
	text := `<tool_call><function=write_file>
<parameter=path>/tmp/a.html</parameter>
<parameter=content>hi</parameter>
</function></tool_call>`

	merged, stripped := mergeXMLToolCalls(api, text)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged call, got %d", len(merged))
	}
	if merged[0].CallID != "call_api" {
		t.Fatalf("expected call_api preserved, got %s", merged[0].CallID)
	}
	if merged[0].Arguments["path"] != "/tmp/a.html" {
		t.Fatalf("unexpected path: %v", merged[0].Arguments["path"])
	}
	if strings.TrimSpace(stripped) != "" {
		t.Fatalf("expected stripped text empty, got %q", stripped)
	}
}

func TestMergeXMLToolCalls_keepsNonEmptyAPIArgs(t *testing.T) {
	api := []service.ToolCall{
		{CallID: "call_api", Name: "write_file", Arguments: map[string]any{"path": "/keep.html", "content": "keep"}},
	}
	text := `<tool_call><function=write_file>
<parameter=path>/other.html</parameter>
<parameter=content>other</parameter>
</function></tool_call>`

	merged, _ := mergeXMLToolCalls(api, text)
	if merged[0].Arguments["path"] != "/keep.html" {
		t.Fatalf("unexpected path: %v", merged[0].Arguments["path"])
	}
}

func TestCoerceXMLArgValue(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"True", true},
		{"false", false},
		{"yes", true},
		{"0", false},
		{"42", 42},
		{"3.14", 3.14},
		{"foo", "foo"},
	}
	for _, c := range cases {
		got := coerceXMLArgValue(c.in)
		if got != c.want {
			t.Errorf("coerceXMLArgValue(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
