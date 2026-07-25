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

func TestExtractXMLToolCalls_anthropicInvokeFormat(t *testing.T) {
	text := `Let me write that file.

<function_calls>
<invoke name="write_file">
<parameter name="path">/home/user/profile.html</parameter>
<parameter name="content"><!DOCTYPE html>
<html><body>Hello</body></html></parameter>
</invoke>
</function_calls>

Done.`

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
	content, _ := calls[0].Arguments["content"].(string)
	if !strings.Contains(content, "<!DOCTYPE html>") {
		t.Fatalf("content missing doctype: %q", content)
	}
	if strings.Contains(stripped, "<function_calls>") {
		t.Fatalf("stripped text still contains invoke xml: %q", stripped)
	}
	if !strings.Contains(stripped, "Let me write") || !strings.Contains(stripped, "Done.") {
		t.Fatalf("stripped text lost surrounding text: %q", stripped)
	}
}

func TestExtractXMLToolCalls_toolUseBareFormat(t *testing.T) {
	text := `I'll search for that.

<tool_use>
<name>search_docs</name>
<parameters>
<query>kafka configuration</query>
<limit>5</limit>
</parameters>
</tool_use>

Hope that helps.`

	calls, stripped := extractXMLToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "search_docs" {
		t.Fatalf("expected search_docs, got %s", calls[0].Name)
	}
	if calls[0].Arguments["query"] != "kafka configuration" {
		t.Fatalf("unexpected query: %v", calls[0].Arguments["query"])
	}
	if calls[0].Arguments["limit"] != 5 {
		t.Fatalf("expected limit=5 (int), got %v", calls[0].Arguments["limit"])
	}
	if strings.Contains(stripped, "<tool_use>") {
		t.Fatalf("stripped text still contains tool_use xml: %q", stripped)
	}
}

func TestExtractXMLToolCalls_multipleFormatsMixed(t *testing.T) {
	text := `First I'll search, then write.

<function_calls>
<invoke name="search_docs">
<parameter name="query">test</parameter>
</invoke>
</function_calls>

Now writing.

<tool_use>
<name>write_file</name>
<parameters>
<path>/tmp/out.txt</path>
<content>hello world</content>
</parameters>
</tool_use>

Done.`

	calls, stripped := extractXMLToolCalls(text)
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Name != "search_docs" {
		t.Fatalf("expected first call search_docs, got %s", calls[0].Name)
	}
	if calls[1].Name != "write_file" {
		t.Fatalf("expected second call write_file, got %s", calls[1].Name)
	}
	if calls[1].Arguments["content"] != "hello world" {
		t.Fatalf("unexpected content: %v", calls[1].Arguments["content"])
	}
	if strings.Contains(stripped, "<function_calls>") || strings.Contains(stripped, "<tool_use>") {
		t.Fatalf("stripped text still contains xml: %q", stripped)
	}
}

func TestRecoverXMLToolCalls_anthropicInvokeFromStream(t *testing.T) {
	result := service.LLMResult{ToolCalls: []service.ToolCall{{
		CallID: "call_api", Name: "write_file", Arguments: map[string]any{},
	}}}
	streamed := `<function_calls>
<invoke name="write_file">
<parameter name="path">/tmp/b.html</parameter>
<parameter name="content">streamed content</parameter>
</invoke>
</function_calls>`

	recovered := RecoverXMLToolCalls(result, streamed)
	if len(recovered.ToolCalls) != 1 {
		t.Fatalf("expected 1 recovered call, got %#v", recovered.ToolCalls)
	}
	if recovered.ToolCalls[0].Arguments["path"] != "/tmp/b.html" {
		t.Fatalf("unexpected args: %#v", recovered.ToolCalls[0].Arguments)
	}
	if recovered.ToolCalls[0].Arguments["content"] != "streamed content" {
		t.Fatalf("unexpected content: %#v", recovered.ToolCalls[0].Arguments)
	}
}

func TestRecoverXMLToolCalls_toolUseFromStream(t *testing.T) {
	result := service.LLMResult{ToolCalls: []service.ToolCall{{
		CallID: "call_api", Name: "search_docs", Arguments: map[string]any{},
	}}}
	streamed := `<tool_use>
<name>search_docs</name>
<parameters>
<query>streamed query</query>
</parameters>
</tool_use>`

	recovered := RecoverXMLToolCalls(result, streamed)
	if len(recovered.ToolCalls) != 1 {
		t.Fatalf("expected 1 recovered call, got %#v", recovered.ToolCalls)
	}
	if recovered.ToolCalls[0].Arguments["query"] != "streamed query" {
		t.Fatalf("unexpected args: %#v", recovered.ToolCalls[0].Arguments)
	}
}

func TestExtractXMLToolCalls_kimiK2SingleCall(t *testing.T) {
	text := `Let me check the weather.

<|tool_calls_section_begin|>
<|tool_call_begin|>functions.get_weather:0<|tool_call_argument_begin|>{"city":"Tokyo"}<|tool_call_end|>
<|tool_calls_section_end|>

Done.`

	calls, stripped := extractXMLToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "get_weather" {
		t.Fatalf("expected get_weather, got %s", calls[0].Name)
	}
	if calls[0].Arguments["city"] != "Tokyo" {
		t.Fatalf("expected city=Tokyo, got %v", calls[0].Arguments["city"])
	}
	if strings.Contains(stripped, "<|tool_calls_section_begin|>") {
		t.Fatalf("stripped text still contains kimi tokens: %q", stripped)
	}
	if !strings.Contains(stripped, "Let me check") || !strings.Contains(stripped, "Done.") {
		t.Fatalf("stripped text lost surrounding text: %q", stripped)
	}
}

func TestExtractXMLToolCalls_kimiK2MultipleCalls(t *testing.T) {
	text := `<|tool_calls_section_begin|>
<|tool_call_begin|>functions.search_docs:0<|tool_call_argument_begin|>{"query":"kafka"}<|tool_call_end|>
<|tool_call_begin|>functions.read_file:1<|tool_call_argument_begin|>{"path":"/tmp/x.txt"}<|tool_call_end|>
<|tool_calls_section_end|>`

	calls, stripped := extractXMLToolCalls(text)
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Name != "search_docs" {
		t.Fatalf("expected first call search_docs, got %s", calls[0].Name)
	}
	if calls[0].Arguments["query"] != "kafka" {
		t.Fatalf("expected query=kafka, got %v", calls[0].Arguments["query"])
	}
	if calls[1].Name != "read_file" {
		t.Fatalf("expected second call read_file, got %s", calls[1].Name)
	}
	if calls[1].Arguments["path"] != "/tmp/x.txt" {
		t.Fatalf("expected path=/tmp/x.txt, got %v", calls[1].Arguments["path"])
	}
	if strings.TrimSpace(stripped) != "" {
		t.Fatalf("expected stripped text empty, got %q", stripped)
	}
}

func TestRecoverXMLToolCalls_kimiFromStream(t *testing.T) {
	result := service.LLMResult{ToolCalls: []service.ToolCall{{
		CallID: "call_api", Name: "get_weather", Arguments: map[string]any{},
	}}}
	streamed := `<|tool_calls_section_begin|>
<|tool_call_begin|>functions.get_weather:0<|tool_call_argument_begin|>{"city":"Paris"}<|tool_call_end|>
<|tool_calls_section_end|>`

	recovered := RecoverXMLToolCalls(result, streamed)
	if len(recovered.ToolCalls) != 1 {
		t.Fatalf("expected 1 recovered call, got %#v", recovered.ToolCalls)
	}
	if recovered.ToolCalls[0].Arguments["city"] != "Paris" {
		t.Fatalf("expected city=Paris, got %#v", recovered.ToolCalls[0].Arguments)
	}
}
