package llm

import (
	"regexp"
	"strconv"
	"strings"

	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/idgen"
)

// Some weaker / non-OpenAI models (e.g. openrouter/xiaomi/mimo-v2.5) emit tool
// calls as XML text instead of native function_call JSON. This parser extracts
// those calls from assistant text so the host can execute them normally.
//
// Supported format:
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
	toolCallBlockRe = regexp.MustCompile(`(?s)(<tool_call>\s*<function=(\w+)>([\s\S]*?)</function>\s*</tool_call>)`)

	// paramRe matches non-content parameters. We handle content separately
	// because its value can span lines and may contain </parameter> substrings.
	paramRe = regexp.MustCompile(`(?s)<parameter=(\w+)>(.*?)</parameter>`)

	// contentRe matches the content parameter, requiring its closing </parameter>
	// to be immediately followed by </function>. Non-greedy backtracking means it
	// finds the actual closing </parameter> even if the content contains the
	// literal string "</parameter>".
	contentRe = regexp.MustCompile(`(?s)<parameter=content>([\s\S]*?)</parameter>\s*</function>`)
)

// extractXMLToolCalls scans text for embedded XML-style tool calls and returns
// any calls found plus the text with those blocks removed.
func extractXMLToolCalls(text string) ([]service.ToolCall, string) {
	matches := toolCallBlockRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil, text
	}

	var calls []service.ToolCall
	var out strings.Builder
	lastEnd := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		full := text[m[2]:m[3]]
		name := text[m[4]:m[5]]

		args := parseXMLArgs(full)
		calls = append(calls, service.ToolCall{
			CallID:    idgen.New("call"),
			Name:      name,
			Arguments: args,
		})

		out.WriteString(text[lastEnd:start])
		lastEnd = end
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
	if strings.Contains(result.Text, "<tool_call>") {
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
