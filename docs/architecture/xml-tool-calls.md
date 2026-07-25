# XML / pipe-delimited tool call parsing

Grounded in `internal/infrastructure/service/llm/xml_tool_parse.go`.

Some models emit tool calls as **text** (XML or pipe-delimited tokens) instead of native OpenAI-compatible `tool_calls` JSON. The parser extracts these from assistant text so the worker can execute them normally.

## When it runs

| Path | When |
|---|---|
| `mergeXMLToolCalls` | Called from `parseResponsesPayload` and `parseChatCompletionPayload` after extracting native JSON tool calls |
| `RecoverXMLToolCalls` | Called from the worker after `llm.Chat` returns, using accumulated streamed text — recovers args missing in native JSON but present in text fallback |

Both paths call `extractXMLToolCalls` which scans text for all supported formats and returns parsed `ToolCall` structs + cleaned text (tool call blocks stripped).

## Supported formats

### 1. Fenced (MiMo / OpenRouter)

```
⟦⟧
<function=write_file>
<parameter=append>False</parameter>
<parameter=path>/home/user/profile.html</parameter>
<parameter=content><!DOCTYPE html>...</parameter>
</function>
⟦⟧
```

- Regex: `toolCallBlockRe`
- Arg parser: `parseXMLArgs` — `<parameter=key>value</parameter>`
- Special handling: `<parameter=content>` can contain nested `<parameter=...>` tags and literal `</parameter>` substrings; `contentRe` anchors on `</parameter>\s*</function>` to find the true closing tag

### 2. Anthropic native (`<function_calls><invoke>`)

```
<function_calls>
<invoke name="search">
<parameter name="query">kafka</parameter>
</invoke>
</function_calls>
```

- Regex: `anthropicInvokeRe`
- Arg parser: `parseInvokeArgs` — `<parameter name="key">value</parameter>`
- Content handling: same nested-parameter logic as Format 1, anchored on `</invoke>`

### 3. Bare `<tool_use>` tags

```
<tool_use>
<name>read_file</name>
<parameters>
<path>/tmp/x.txt</path>
<content>file contents here</content>
</parameters>
</tool_use>
```

- Regex: `toolUseRe`
- Arg parser: `parseToolUseArgs` — bare `<key>value</key>` inside `<parameters>`
- Content handling: `<content>` extracted first, then remaining bare tags

### 4. Kimi K2 pipe-delimited tokens

```
<|tool_calls_section_begin|>
<|tool_call_begin|>functions.get_weather:0<|tool_call_argument_begin|>{"city":"Tokyo"}<|tool_call_end|>
<|tool_calls_section_end|>
```

- Regex: `kimiSectionRe` (section wrapper) + `kimiCallRe` (individual call)
- Header format: `functions.{name}:{idx}` — name extracted via `kimiHeaderRe`
- Arg parser: `parseKimiArgs` — tries JSON first, then `tool_sep` multiline, then bare command fallback

#### Kimi variants

| Variant | Example | Handling |
|---|---|---|
| **Unicode delimiters** | `<｜tool▁calls▁begin｜>` (fullwidth pipe U+FF5C, block underscore U+2581) | `normalizeKimiText` replaces `｜`→`|`, `│`→`|`, `▁`→`_`, `‗`→`_` |
| **Spaced tokens** | `<  \|  tool_call_begin  \|  >` | `kimiSpacedTokenRe` collapses to `<\|tool_call_begin\|>` |
| **Redacted tokens** | `<\|redacted_tool_call_begin\|>`, `<\|redacted_tool_calls_section_begin\|>` | Regex alternation covers `redacted_*` prefix |
| **Kimi suffix** | `<\|redacted_tool_call_begin_kimi\|>` | Regex covers `_kimi` suffix on begin/end |
| **Section without `_section`** | `<\|tool_calls_begin\|>...<\|tool_calls_end\|>` | Regex alternation covers both `tool_calls_begin` and `tool_calls_section_begin` |
| **`tool_sep` args** | `key\nvalue<\|tool_sep\|>key2\nvalue2` | `parseKimiMultiline` splits on `tool_sep` and parses key-value pairs |
| **Bare command** | `echo hello world` (no JSON, no key-value) | Fallback wraps in `{"input": raw}` |

#### Normalization

All text is normalized via `normalizeKimiText` **before** any regex matching. This ensures Unicode delimiters and spaced tokens are converted to ASCII so regexes match uniformly. Normalization is applied to the full text (not just Kimi sections) because it only touches `<|...|>` token patterns — non-Kimi text is unaffected.

#### Multiple calls per section

A single Kimi section can contain multiple `<|tool_call_begin|>...<|tool_call_end|>` blocks. Each is expanded into a separate `blockMatch` entry sharing the same section span. The output loop emits all calls but only strips the section once (the first call advances `lastEnd`; subsequent calls with `start < lastEnd` emit without re-stripping).

## Positional merge

All formats are scanned independently, then merged by `start` offset (`sort.SliceStable`). This preserves text order across mixed formats in the same response.

## `mergeXMLToolCalls` — native + XML merge

1. Extract XML/pipe calls from text
2. For each native JSON tool call, find a matching XML call by name
3. If the native call has empty arguments, fill from the XML call
4. Append any unmatched XML calls (no native equivalent)
5. Return merged calls + stripped text

## `RecoverXMLToolCalls` — stream recovery

After streaming completes, the worker calls `RecoverXMLToolCalls(result, streamedText)`:

1. `mergeXMLToolCalls` on the streamed text
2. If calls changed or text was cleaned → update `result.ToolCalls`
3. If `result.Text` contains any format marker (fenced, `<function_calls>`, `<tool_use>`, `<|tool_calls_section_begin|>`, `<|tool_calls_begin|>`, `<|redacted_tool_calls`) → replace with cleaned text

Marker detection runs on normalized text (via `normalizeKimiText`) so Unicode Kimi variants are caught.

## Value coercion

`coerceXMLArgValue` converts string values to Go types:
- `true`/`yes`/`1` → `bool true`
- `false`/`no`/`0` → `bool false`
- Integer string → `int`
- Float string → `float64`
- Otherwise → `string`

## Tests

- `xml_tool_parse_test.go` — all formats, edge cases, stream recovery
- `stream_xml_tool_test.go` — XML + native JSON in streaming
- `client_test.go` — end-to-end merge in response parsing

## Related

- [LLM providers](llm-providers.md) — where `mergeXMLToolCalls` is called in response parsing
- [Turn loop](turn-loop.md) — where `RecoverXMLToolCalls` runs in the worker
- [Realtime streaming](realtime-streaming.md) — streamed text accumulation
