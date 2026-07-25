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
//     Variants: Unicode delimiters (｜▁), spaced tokens, redacted_* prefix,
//     tool_calls_begin (no _section), tool_sep multiline args, bare command.
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

	// Format 4 (Kimi K2): section wrappers. Covers tool_calls_section_begin,
	// tool_calls_begin, and redacted_* variants. Text is pre-normalized so
	// Unicode delimiters (｜▁) and spaced tokens are already ASCII.
	kimiSectionRe = regexp.MustCompile(`(?s)(<\|(?:tool_calls_section_begin|tool_calls_begin|redacted_tool_calls_section_begin|redacted_tool_calls_begin)\|>[\s\S]*?<\|(?:tool_calls_section_end|tool_calls_end|redacted_tool_calls_section_end|redacted_tool_calls_end)\|>)`)

	// Format 4 individual call: <|tool_call_begin|>header<sep>args<|tool_call_end|>
	// Sep can be tool_call_argument_begin, tool_sep, or redacted variants.
	kimiCallRe = regexp.MustCompile(`(?s)<\|(?:tool_call_begin|redacted_tool_call_begin(?:_kimi)?)\|>\s*(.*?)\s*<\|(?:tool_call_argument_begin|tool_sep|redacted_tool_sep|redacted_tool_call_argument_begin)\|>(.*?)<\|(?:tool_call_end|redacted_tool_call_end(?:_kimi)?)\|>`)

	// Format 4 header: functions.{name}:{idx}
	kimiHeaderRe = regexp.MustCompile(`functions\.(\w+):\d+`)

	// Format 4 tool_sep splitter for multiline args (applied after normalization)
	kimiSepRe = regexp.MustCompile(`<\|(?:tool_sep|redacted_tool_sep|redacted_tool_call_argument_begin)\|>\s*`)

	// Format 4 spaced token normalization: <  |  foo  |  > → <|foo|>
	kimiSpacedTokenRe = regexp.MustCompile(`<\s*\|\s*([a-z0-9_]+)\s*\|\s*>`)

	// Format 4 section strip (for cleaning visible text)
	kimiStripRe = regexp.MustCompile(`(?s)<\|(?:tool_calls_begin|tool_calls_end|tool_calls_section_begin|tool_calls_section_end|tool_call_begin|tool_call_end|tool_call_argument_begin|tool_sep|redacted_tool_calls_begin|redacted_tool_calls_end|redacted_tool_calls_section_begin|redacted_tool_calls_section_end|redacted_tool_call_begin|redacted_tool_call_end|redacted_tool_call_begin_kimi|redacted_tool_call_end_kimi|redacted_tool_sep|redacted_tool_call_argument_begin)\|>`)
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

	// Normalize Kimi Unicode delimiters + spaced tokens up front so all
	// regexes work on the same byte offsets. Non-Kimi text is unaffected
	// (normalization only touches fullwidth pipe/underscore and < | > spacing).
	normText := normalizeKimiText(text)

	var all []blockMatch

	for _, m := range toolCallBlockRe.FindAllStringSubmatchIndex(normText, -1) {
		all = append(all, blockMatch{
			start: m[0], end: m[1],
			name:      normText[m[4]:m[5]],
			inner:     normText[m[6]:m[7]],
			argParser: parseXMLArgs,
		})
	}

	for _, m := range anthropicInvokeRe.FindAllStringSubmatchIndex(normText, -1) {
		all = append(all, blockMatch{
			start: m[0], end: m[1],
			name:      normText[m[4]:m[5]],
			inner:     normText[m[6]:m[7]],
			argParser: parseInvokeArgs,
		})
	}

	for _, m := range toolUseRe.FindAllStringSubmatchIndex(normText, -1) {
		all = append(all, blockMatch{
			start: m[0], end: m[1],
			name:      normText[m[4]:m[5]],
			inner:     normText[m[6]:m[7]],
			argParser: parseToolUseArgs,
		})
	}

	// Kimi K2: scan normalized text for section + call tokens.
	for _, sec := range kimiSectionRe.FindAllStringSubmatchIndex(normText, -1) {
		secStart, secEnd := sec[0], sec[1]
		sectionText := normText[secStart:secEnd]
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
		return nil, normText
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
		out.WriteString(normText[lastEnd:m.start])
		lastEnd = m.end
	}
	out.WriteString(normText[lastEnd:])
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

// parseKimiArgs parses Kimi K2 tool call arguments. The args can be:
//   - JSON object (e.g. {"city":"Tokyo"})
//   - tool_sep multiline key-value pairs (e.g. key\nvalue<|tool_sep|>key2\nvalue2)
//   - Bare command text for Shell/exec tools
func parseKimiArgs(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	// Try JSON first.
	if strings.HasPrefix(raw, "{") {
		var args map[string]any
		if err := json.Unmarshal([]byte(raw), &args); err == nil {
			// Check for nested "input" that is multiline key-value.
			if inner, ok := args["input"].(string); ok && shouldParseKimiMultiline(inner) {
				if kv := parseKimiMultiline(inner); len(kv) > 0 {
					return kv
				}
			}
			return args
		}
	}
	// Try tool_sep multiline key-value.
	if shouldParseKimiMultiline(raw) {
		if kv := parseKimiMultiline(raw); len(kv) > 0 {
			return kv
		}
	}
	// Fallback: bare command for exec/Shell tools.
	return map[string]any{"input": raw}
}

// shouldParseKimiMultiline checks if raw contains tool_sep markers or
// matches the multiline key\nvalue pattern.
func shouldParseKimiMultiline(raw string) bool {
	if kimiSepRe.MatchString(raw) {
		return true
	}
	return regexp.MustCompile(`(?m)^[a-zA-Z_][\w.-]*\n`).MatchString(raw)
}

// parseKimiMultiline parses tool_sep delimited key-value pairs.
// Format: key1\nvalue1<|tool_sep|>key2\nvalue2
func parseKimiMultiline(raw string) map[string]any {
	parts := kimiSepRe.Split(strings.TrimSpace(raw), -1)
	out := map[string]any{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		nl := strings.Index(part, "\n")
		if nl <= 0 {
			continue
		}
		key := strings.TrimSpace(part[:nl])
		value := strings.TrimSpace(part[nl+1:])
		if key == "" {
			continue
		}
		out[key] = coerceXMLArgValue(value)
	}
	return out
}

// normalizeKimiText normalizes Unicode delimiters and spaced tokens in Kimi
// tool call syntax to their ASCII equivalents so regexes can match uniformly.
func normalizeKimiText(text string) string {
	text = strings.ReplaceAll(text, "\uFF5C", "|") // fullwidth pipe
	text = strings.ReplaceAll(text, "\u2502", "|") // box drawings light vertical
	text = strings.ReplaceAll(text, "\u2581", "_") // lower one eighth block
	text = strings.ReplaceAll(text, "\u2017", "_") // double low line
	return kimiSpacedTokenRe.ReplaceAllString(text, "<|$1|>")
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
	normResultText := normalizeKimiText(result.Text)
	if strings.Contains(normResultText, "<tool_call>") ||
		strings.Contains(normResultText, "<function_calls>") ||
		strings.Contains(normResultText, "<tool_use>") ||
		strings.Contains(normResultText, "<|tool_calls_section_begin|>") ||
		strings.Contains(normResultText, "<|tool_calls_begin|>") ||
		strings.Contains(normResultText, "<|redacted_tool_calls") {
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
