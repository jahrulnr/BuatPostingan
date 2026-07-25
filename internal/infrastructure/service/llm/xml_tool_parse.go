package llm

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/idgen"
)

// Some weaker / non-OpenAI models (e.g. openrouter/xiaomi/mimo-v2.5, Anthropic
// via OpenAI-compatible proxies) emit tool calls as XML text instead of native
// function_call JSON. This parser extracts those calls from assistant text so
// the host can execute them normally.
//
// Supported formats:
//  1. Fenced (MiMo / OpenRouter)
//  2. Anthropic native: <function_calls><invoke name="...">
//  3. tool_use bare tags: <tool_use><name>...</name>
//  4. Kimi K2 pipe tokens: <|tool_calls_section_begin|>...<|tool_calls_section_end|>
//
//	<tool_call>
//	<function=write_file>
//	<parameter=append>False</parameter>
//	<parameter=path>/home/user/profile.html</parameter>
//	<parameter=content><!DOCTYPE html>...</parameter>
//	</function>
//	</tool_call>

var (
	// toolCallBlockRe matches the whole <tool_call>...</tool_call> block.
	// It captures: 1 = full block, 2 = function name, 3 = inner parameters.
	// The non-greedy ([\s\S]*?) combined with the fixed closing tags means it
	// correctly skips any </function> or </tool_call> substrings that appear
	// inside a parameter value.
	toolCallBlockRe = regexp.MustCompile(`(?s)(<tool_call>\s*<function=(\w+)>([\s\S]*?</function>)\s*</tool_call>)`)

	// paramRe matches non-content parameters. We handle content separately
	// because its value can span lines and may contain </parameter> substrings.
	paramRe = regexp.MustCompile(`(?s)<parameter=(\w+)>(.*?)</parameter>`)

	// Format 2: <function_calls><invoke name="...">...</invoke></function_calls>
	anthropicInvokeRe = regexp.MustCompile(`(?s)(<function_calls>\s*<invoke\s+name="(\w+)">([\s\S]*?</invoke>)\s*</function_calls>)`)

	// Format 3: <tool_use><name>...</name><parameters>...</parameters></tool_use>
	toolUseRe = regexp.MustCompile(`(?s)(<tool_use>\s*<name>(\w+)</name>\s*<parameters>([\s\S]*?</parameters>)\s*</tool_use>)`)

	// Format 2 params: <parameter name="key">value</parameter>
	paramNameRe = regexp.MustCompile(`(?s)<parameter\s+name="(\w+)">(.*?)</parameter>`)

	// Format 2 content: <parameter name="content">...</parameter></invoke>
	contentInvokeRe = regexp.MustCompile(`(?s)<parameter\s+name="content">([\s\S]*?)</parameter>\s*</invoke>`)

	// Format 3 content: <content>...</content></parameters>
	contentToolUseRe = regexp.MustCompile(`(?s)<content>([\s\S]*?)</content>\s*</parameters>`)

	// Format 3 bare params: <key>value</key>
	bareParamRe = regexp.MustCompile(`(?s)<(\w+)>([\s\S]*?)</\w+>`)

	// contentRe matches the content parameter, requiring its closing </parameter>
	// to be immediately followed by </function>. Non-greedy backtracking means it
	// finds the actual closing </parameter> even if the content contains the
	// literal string "</parameter>".
	contentRe = regexp.MustCompile(`(?s)<parameter=content>([\s\S]*?)</parameter>\s*</function>`)

	// Format 4 (Kimi K2): <|tool_calls_section_begin|>...<|tool_calls_section_end|>
	kimiSectionRe = regexp.MustCompile(`(?s)(<\|tool_calls_section_begin\|>[\s\S]*?<\|tool_calls_section_end\|>)`)

	// Format 4 individual call: <|tool_call_begin|>header<|tool_call_argument_begin|>args<|tool_call_end|>
	kimiCallRe = regexp.MustCompile(`(?s)<\|tool_call_begin\|>\s*(.*?)\s*<\|tool_call_argument_begin\|>(.*?)<\|tool_call_end\|>`)

	// Format 4 header: functions.{name}:{idx}
	kimiHeaderRe = regexp.MustCompile(`functions\.(\w+):\d+`)
)

// extractXMLToolCalls scans text for embedded XML-style tool calls across all
// supported formats and returns any calls found plus the text with those blocks
// removed. Matches from all formats are merged in positional order.
func extractXMLToolCalls(text string) ([]service.ToolCall, string) {
	type blockMatch struct {
		start, end int
		name       string
		inner      string
		argParser  func(string) map[string]any
	}

	var all []blockMatch

	for _, m := range toolCallBlockRe.FindAllStringSubmatchIndex(text, -1) {
		all = append(all, blockMatch{
			start: m[0], end: m[1],
			name:      text[m[4]:m[5]],
			inner:     text[m[6]:m[7]],
			argParser: parseXMLArgs,
		})
	}

	for _, m := range anthropicInvokeRe.FindAllStringSubmatchIndex(text, -1) {
		all = append(all, blockMatch{
			start: m[0], end: m[1],
			name:      text[m[4]:m[5]],
			inner:     text[m[6]:m[7]],
			argParser: parseInvokeArgs,
		})
	}

	for _, m := range toolUseRe.FindAllStringSubmatchIndex(text, -1) {
		all = append(all, blockMatch{
			start: m[0], end: m[1],
			name:      text[m[4]:m[5]],
			inner:     text[m[6]:m[7]],
			argParser: parseToolUseArgs,
		})
	}

	// Kimi K2: one section can contain multiple calls, so we expand them
	// into individual blockMatch entries sharing the same section span.
	for _, sec := range kimiSectionRe.FindAllStringSubmatchIndex(text, -1) {
		secStart, secEnd := sec[0], sec[1]
		sectionText := text[secStart:secEnd]
		for _, kc := range kimiCallRe.FindAllStringSubmatch(sectionText, -1) {
			header := strings.TrimSpace(kc[1])
			name := header
			if hm := kimiHeaderRe.FindStringSubmatch(header); hm != nil {
				name = hm[1]
			}
			all = append(all, blockMatch{
				start: secStart, end: secEnd,
				name:      name,
				inner:     strings.TrimSpace(kc[2]),
				argParser: parseKimiArgs,
			})
		}
	}

	if len(all) == 0 {
		return nil, text
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].start < all[j].start })

	var calls []service.ToolCall
	var out strings.Builder
	lastEnd := 0
	for _, m := range all {
		if m.start < lastEnd {
			// Kimi: multiple calls share the same section span; emit the
			// call but don't write text or advance lastEnd.
			calls = append(calls, service.ToolCall{
				CallID:    idgen.New("call"),
				Name:      m.name,
				Arguments: m.argParser(m.inner),
			})
			continue
		}
		calls = append(calls, service.ToolCall{
			CallID:    idgen.New("call"),
			Name:      m.name,
			Arguments: m.argParser(m.inner),
		})
		out.WriteString(text[lastEnd:m.start])
		lastEnd = m.end
	}
	out.WriteString(text[lastEnd:])
	return calls, out.String()
}

func parseXMLArgs(full string) map[string]any {
	args := map[string]any{}

	for _, m := range paramRe.FindAllStringSubmatch(full, -1) {
		key := m[1]
		if key == "content" {
			continue
		}
		args[key] = coerceXMLArgValue(strings.TrimSpace(m[2]))
	}

	if m := contentRe.FindStringSubmatch(full); m != nil {
		content, nested := stripNestedXMLParameters(m[1])
		for key, value := range nested {
			if _, exists := args[key]; !exists {
				args[key] = value
			}
		}
		args["content"] = content
	}

	return args
}

// parseInvokeArgs parses <parameter name="key">value</parameter> (Format 2).
func parseInvokeArgs(full string) map[string]any {
	args := map[string]any{}

	for _, m := range paramNameRe.FindAllStringSubmatch(full, -1) {
		key := m[1]
		if key == "content" {
			continue
		}
		args[key] = coerceXMLArgValue(strings.TrimSpace(m[2]))
	}

	if m := contentInvokeRe.FindStringSubmatch(full); m != nil {
		content, nested := stripNestedNamedParameters(m[1])
		for key, value := range nested {
			if _, exists := args[key]; !exists {
				args[key] = value
			}
		}
		args["content"] = content
	}

	return args
}

// parseToolUseArgs parses bare <key>value</key> tags (Format 3).
func parseToolUseArgs(inner string) map[string]any {
	args := map[string]any{}

	cleanInner := inner
	if m := contentToolUseRe.FindStringSubmatchIndex(inner); m != nil {
		args["content"] = inner[m[2]:m[3]]
		cleanInner = inner[:m[0]] + inner[m[1]:]
	}

	for _, m := range bareParamRe.FindAllStringSubmatch(cleanInner, -1) {
		key := m[1]
		if key == "name" {
			continue
		}
		if _, exists := args[key]; !exists {
			args[key] = coerceXMLArgValue(strings.TrimSpace(m[2]))
		}
	}

	return args
}

// parseKimiArgs parses Kimi K2 tool call arguments, which are a JSON string
// (e.g. {"city":"Tokyo"}). Falls back to empty map on parse error.
func parseKimiArgs(raw string) map[string]any {
	args := map[string]any{}
	if raw == "" {
		return args
	}
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return args
	}
	return args
}

func stripNestedXMLParameters(content string) (string, map[string]any) {
	nested := map[string]any{}
	clean := paramRe.ReplaceAllStringFunc(content, func(raw string) string {
		m := paramRe.FindStringSubmatch(raw)
		if len(m) != 3 {
			return raw
		}
		key := m[1]
		if key == "content" {
			return raw
		}
		nested[key] = coerceXMLArgValue(strings.TrimSpace(m[2]))
		return ""
	})
	return clean, nested
}

func stripNestedNamedParameters(content string) (string, map[string]any) {
	nested := map[string]any{}
	clean := paramNameRe.ReplaceAllStringFunc(content, func(raw string) string {
		m := paramNameRe.FindStringSubmatch(raw)
		if len(m) != 3 {
			return raw
		}
		key := m[1]
		if key == "content" {
			return raw
		}
		nested[key] = coerceXMLArgValue(strings.TrimSpace(m[2]))
		return ""
	})
	return clean, nested
}

func coerceXMLArgValue(s string) any {
	switch strings.ToLower(s) {
	case "true", "yes", "1":
		return true
	case "false", "no", "0":
		return false
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

func RecoverXMLToolCalls(result service.LLMResult, streamedText string) service.LLMResult {
	calls, clean := mergeXMLToolCalls(result.ToolCalls, streamedText)
	if len(calls) == len(result.ToolCalls) && clean == streamedText {
		return result
	}
	result.ToolCalls = calls
	if strings.Contains(result.Text, "<tool_call>") ||
		strings.Contains(result.Text, "<function_calls>") ||
		strings.Contains(result.Text, "<tool_use>") ||
		strings.Contains(result.Text, "<|tool_calls_section_begin|>") {
		result.Text = clean
	}
	return result
}

func mergeXMLToolCalls(api []service.ToolCall, text string) ([]service.ToolCall, string) {
	xml, stripped := extractXMLToolCalls(text)
	if len(xml) == 0 {
		return api, text
	}

	used := make([]bool, len(xml))
	out := make([]service.ToolCall, 0, len(api)+len(xml))

	for _, tc := range api {
		merged := tc
		for i, xc := range xml {
			if used[i] || xc.Name != tc.Name {
				continue
			}
			used[i] = true
			if len(tc.Arguments) == 0 {
				merged.Arguments = xc.Arguments
			}
			break
		}
		out = append(out, merged)
	}

	for i, xc := range xml {
		if !used[i] {
			out = append(out, xc)
		}
	}

	return out, stripped
}
