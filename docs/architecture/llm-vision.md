# LLM vision (multimodal images)

Grounded in `internal/infrastructure/worker/vision.go`, `internal/infrastructure/service/llm/vision_policy.go`, and `BP_LLM_VISION`.

## Config

| Value | Behavior |
|---|---|
| `auto` (default) | Probe OpenRouter-style `GET {base}/models` for `architecture.input_modalities` containing `image` (cached per baseURL+model). If probe fails, use heuristics (e.g. `mimo`, `gpt-4o`, `claude-*`, `gemini`, `*-vl`). |
| `on` | Always attach image pixels when under size caps. |
| `off` | Metadata only — never attach `image_url` / `input_image` parts. |

```bash
BP_LLM_VISION=auto   # recommended
# BP_LLM_VISION=on
# BP_LLM_VISION=off
```

## Request shapes

Worker builds **chat-completions** content; the LLM client maps to Responses when `API=responses`.

**Chat**

```json
{
  "role": "user",
  "content": [
    { "type": "text", "text": "…attachments metadata…" },
    { "type": "image_url", "image_url": { "url": "data:image/png;base64,…" } }
  ]
}
```

**Responses** (via `toResponsesUserContent`)

```json
{
  "role": "user",
  "content": [
    { "type": "input_text", "text": "…" },
    { "type": "input_image", "image_url": "data:image/png;base64,…" }
  ]
}
```

Pixels always come from the **attachment store** as `data:` URLs — never from arbitrary remote URLs (SSRF).

## Caps

| Cap | Value |
|---|---|
| Per image | 4 MiB (`tools.MaxVisionImageBytes`) |
| Per user message | 4 images |
| Per rebuilt context | 8 images |

## Tools

`read_image` returns metadata; `vision_available` / `content_provided_to_model` are true only when bytes load under the size cap **and** the vision gate allows pixels.

## MiMo / OpenRouter

`xiaomi/mimo-v2.5` reports `input_modalities: text,image,audio,video` (omnimodal). Use it with `BP_LLM_VISION=auto` or `on`. Text-only models (e.g. DeepSeek chat) get metadata-only under `auto`.

OpenRouter may return `404 No endpoints found that support image input` if images are sent to a text-only model — the gate prevents that when detection works.

## Related

- [LLM providers](llm-providers.md)
- [Architecture](README.md)
- OpenRouter image inputs: https://openrouter.ai/docs/guides/overview/multimodal/image-understanding
- OpenAI vision: https://developers.openai.com/api/docs/guides/images-vision
