package llm

import (
	"strings"
	"testing"

	"buatpostingan/internal/config"
)

func TestChatStreamKeepsXMLToolContentAlongsideNativeCall(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"<tool_call><function=write_file>"}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"<parameter=path>/tmp/profile.html</parameter><parameter=content>hello</parameter></function></tool_call>"}}]}`,
		"",
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"write_file","arguments":""}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{}}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")

	payload, err := parseSSEToPayload(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	res := parseChatCompletionPayload(config.LLMProvider{ID: "P", Model: "m", API: "chat"}, payload)
	if len(res.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %#v", res.ToolCalls)
	}
	if res.ToolCalls[0].Name != "write_file" {
		t.Fatalf("name=%q", res.ToolCalls[0].Name)
	}
	if res.ToolCalls[0].Arguments["path"] != "/tmp/profile.html" {
		t.Fatalf("path=%v args=%#v", res.ToolCalls[0].Arguments["path"], res.ToolCalls[0].Arguments)
	}
	if res.ToolCalls[0].Arguments["content"] != "hello" {
		t.Fatalf("content=%v args=%#v", res.ToolCalls[0].Arguments["content"], res.ToolCalls[0].Arguments)
	}
	if strings.Contains(res.Text, "<tool_call>") {
		t.Fatalf("raw XML leaked into final text: %q", res.Text)
	}
}
