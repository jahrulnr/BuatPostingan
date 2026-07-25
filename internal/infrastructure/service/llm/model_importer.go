package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/apperr"
)

var _ service.ProviderModelImporter = (*ModelImporter)(nil)

// ModelImporter fetches model ids from an OpenAI- or Anthropic-compatible
// /models endpoint. It supports OpenAI, Anthropic, OpenRouter, and generic
// OpenAI-compatible providers that return a top-level `data` array.
type ModelImporter struct {
	http *http.Client
	// MaxPages caps pagination depth to avoid runaway upstream loops.
	MaxPages int
	// PageSize is the `limit` parameter for paginated endpoints.
	PageSize int
}

// NewModelImporter creates a ModelImporter with sane defaults.
func NewModelImporter() *ModelImporter {
	return &ModelImporter{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		MaxPages: 10,
		PageSize: 1000,
	}
}

// ImportModels calls the provider's /models endpoint and returns a deduplicated,
// sorted list of model ids. It follows simple pagination for Anthropic and
// OpenRouter providers.
func (m *ModelImporter) ImportModels(ctx context.Context, provider entity.SettingsProvider) ([]entity.SettingsModel, error) {
	base := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	if base == "" {
		return nil, apperr.Validation("provider base_url required")
	}

	target, err := url.Parse(base + "/models")
	if err != nil {
		return nil, apperr.Wrap(500, apperr.CodeInternal, "parse base_url", err)
	}

	key := strings.TrimSpace(provider.APIKey)
	seen := make(map[string]struct{})
	var out []entity.SettingsModel

	for page := 0; page < m.MaxPages; page++ {
		items, next, err := m.fetchPage(ctx, target, key)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			id := strings.TrimSpace(it.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, it.ToEntity())
		}
		if next == "" {
			break
		}
		nextURL, err := target.Parse(next)
		if err != nil {
			return nil, apperr.Wrap(500, apperr.CodeInternal, "parse next page url", err)
		}
		target = nextURL
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// modelListItem is the smallest common shape across OpenAI, Anthropic, and
// OpenRouter /models responses. Extra fields are parsed best-effort and
// carried into SettingsModel via ToEntity.
type modelListItem struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`

	// OpenRouter fields.
	ContextLength   int            `json:"context_length"`
	Architecture    *archInfo      `json:"architecture"`
	SupportedParams []string       `json:"supported_parameters"`
	Reasoning       *reasoningInfo `json:"reasoning"`

	// Anthropic fields.
	MaxInputTokens int      `json:"max_input_tokens"`
	MaxTokens      int      `json:"max_tokens"`
	Capabilities   *capInfo `json:"capabilities"`
}

type archInfo struct {
	Modality         string   `json:"modality"`          // "text->text", "text+image->text"
	InputModalities  []string `json:"input_modalities"`  // ["text", "image", "file"]
	OutputModalities []string `json:"output_modalities"` // ["text"]
}

type reasoningInfo struct {
	SupportedEfforts []string `json:"supported_efforts"`
	DefaultEffort    string   `json:"default_effort"`
	Mandatory        bool     `json:"mandatory"`
}

type capInfo struct {
	ImageInput        *capSupport `json:"image_input"`
	PDFInput          *capSupport `json:"pdf_input"`
	Effort            *effortCap  `json:"effort"`
	StructuredOutputs *capSupport `json:"structured_outputs"`
}

type capSupport struct {
	Supported bool `json:"supported"`
}

type effortCap struct {
	Supported bool        `json:"supported"`
	Low       *capSupport `json:"low"`
	Medium    *capSupport `json:"medium"`
	High      *capSupport `json:"high"`
	Max       *capSupport `json:"max"`
	Xhigh     *capSupport `json:"xhigh"`
}

func (it modelListItem) Label() string {
	if it.DisplayName != "" {
		return it.DisplayName
	}
	if it.Name != "" {
		return it.Name
	}
	return ""
}

// ToEntity converts a raw API item into a SettingsModel, populating metadata
// from whichever provider-specific fields are present.
func (it modelListItem) ToEntity() entity.SettingsModel {
	m := entity.SettingsModel{
		ID:          strings.TrimSpace(it.ID),
		Label:       strings.TrimSpace(it.Label()),
		Description: strings.TrimSpace(it.Description),
	}
	if m.Label == "" {
		m.Label = m.ID
	}

	// Context window — OpenRouter uses context_length, Anthropic uses max_input_tokens.
	if it.ContextLength > 0 {
		m.ContextWindow = it.ContextLength
	} else if it.MaxInputTokens > 0 {
		m.ContextWindow = it.MaxInputTokens
	}

	// Max output tokens — Anthropic uses max_tokens.
	if it.MaxTokens > 0 {
		m.MaxOutput = it.MaxTokens
	}

	// Input/output modalities.
	if it.Architecture != nil {
		m.InputModes = normalizeModes(it.Architecture.InputModalities)
		m.OutputModes = normalizeModes(it.Architecture.OutputModalities)
		// Fallback: parse modality shorthand like "text+image->text".
		if len(m.InputModes) == 0 && it.Architecture.Modality != "" {
			m.InputModes, m.OutputModes = parseModalityShorthand(it.Architecture.Modality)
		}
	}
	// Anthropic capabilities.
	if it.Capabilities != nil {
		if it.Capabilities.ImageInput != nil && it.Capabilities.ImageInput.Supported {
			m.InputModes = addIfMissing(m.InputModes, "image")
		}
		if it.Capabilities.PDFInput != nil && it.Capabilities.PDFInput.Supported {
			m.InputModes = addIfMissing(m.InputModes, "pdf")
		}
		if it.Capabilities.Effort != nil && it.Capabilities.Effort.Supported {
			m.EffortLevels = collectEffortLevels(it.Capabilities.Effort)
		}
	}

	// Effort levels — OpenRouter reasoning object.
	if it.Reasoning != nil && len(it.Reasoning.SupportedEfforts) > 0 {
		m.EffortLevels = normalizeEfforts(it.Reasoning.SupportedEfforts)
	}

	// Tools support — check supported_parameters for "tools" or "tool_choice".
	for _, p := range it.SupportedParams {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "tools" || p == "tool_choice" || p == "parallel_tool_calls" {
			m.SupportsTools = true
			break
		}
	}

	return m
}

func normalizeModes(modes []string) []string {
	var out []string
	for _, m := range modes {
		m = strings.ToLower(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		out = addIfMissing(out, m)
	}
	return out
}

func addIfMissing(slice []string, v string) []string {
	for _, s := range slice {
		if strings.EqualFold(s, v) {
			return slice
		}
	}
	return append(slice, v)
}

func parseModalityShorthand(s string) (inputs, outputs []string) {
	s = strings.TrimSpace(s)
	arrow := strings.Index(s, "->")
	if arrow < 0 {
		return nil, nil
	}
	left := strings.TrimSpace(s[:arrow])
	right := strings.TrimSpace(s[arrow+2:])
	for _, part := range strings.Split(left, "+") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			inputs = addIfMissing(inputs, part)
		}
	}
	for _, part := range strings.Split(right, "+") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			outputs = addIfMissing(outputs, part)
		}
	}
	return inputs, outputs
}

func collectEffortLevels(ec *effortCap) []string {
	var out []string
	if ec.Low != nil && ec.Low.Supported {
		out = append(out, "low")
	}
	if ec.Medium != nil && ec.Medium.Supported {
		out = append(out, "medium")
	}
	if ec.High != nil && ec.High.Supported {
		out = append(out, "high")
	}
	if ec.Max != nil && ec.Max.Supported {
		out = append(out, "max")
	}
	if ec.Xhigh != nil && ec.Xhigh.Supported {
		out = append(out, "xhigh")
	}
	return out
}

func normalizeEfforts(efforts []string) []string {
	var out []string
	for _, e := range efforts {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		out = addIfMissing(out, e)
	}
	return out
}

// modelListPage is a normalized first page plus pagination hint.
type modelListPage struct {
	Items      []modelListItem
	NextURL    string
	LastID     string
	HasMore    bool
	TotalCount int
}

func (m *ModelImporter) fetchPage(ctx context.Context, target *url.URL, key string) ([]modelListItem, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "", apperr.Wrap(500, apperr.CodeInternal, "build request", err)
	}

	req.Header.Set("Accept", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("x-api-key", key)
	}

	resp, err := m.http.Do(req)
	if err != nil {
		return nil, "", apperr.Wrap(502, apperr.CodeUpstream, "fetch models", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, "", apperr.New(resp.StatusCode, apperr.CodeUpstream, fmt.Sprintf("models endpoint returned %d: %s", resp.StatusCode, string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, "", apperr.Wrap(500, apperr.CodeInternal, "read models response", err)
	}

	var raw struct {
		Object     string          `json:"object"`
		Data       []modelListItem `json:"data"`
		HasMore    bool            `json:"has_more"`
		FirstID    string          `json:"first_id"`
		LastID     string          `json:"last_id"`
		TotalCount int             `json:"total_count"`
		Links      struct {
			Next string `json:"next"`
		} `json:"links"`
	}
	var arr []modelListItem
	switch {
	case json.Unmarshal(body, &raw) == nil:
		// Standard { data: [...] } envelope.
	case len(bytes.TrimSpace(body)) > 0 && bytes.TrimSpace(body)[0] == '[' && json.Unmarshal(body, &arr) == nil:
		raw.Data = arr
	default:
		return nil, "", apperr.Wrap(502, apperr.CodeUpstream, "decode models response", fmt.Errorf("invalid JSON shape"))
	}

	items := make([]modelListItem, 0, len(raw.Data))
	for _, it := range raw.Data {
		if strings.TrimSpace(it.ID) == "" {
			continue
		}
		// OpenAI uses object=="model", Anthropic uses type=="model".
		if it.Object != "" && !strings.EqualFold(it.Object, "model") {
			continue
		}
		if it.Type != "" && !strings.EqualFold(it.Type, "model") {
			continue
		}
		items = append(items, it)
	}

	next := ""
	switch {
	case raw.Links.Next != "":
		// OpenRouter / newer OpenAI-compatible paginated links.
		next = raw.Links.Next
	case raw.HasMore && raw.LastID != "":
		// Anthropic cursor pagination: build next URL from a copy.
		nextURL := *target
		q := nextURL.Query()
		q.Set("after_id", raw.LastID)
		if m.PageSize > 0 {
			q.Set("limit", strconv.Itoa(m.PageSize))
		}
		nextURL.RawQuery = q.Encode()
		next = nextURL.String()
	}

	return items, next, nil
}
