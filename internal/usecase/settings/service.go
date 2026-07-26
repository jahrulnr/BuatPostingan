package settings

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/repository"
	"buatpostingan/internal/domain/service"
	"buatpostingan/internal/pkg/apperr"
	"buatpostingan/internal/pkg/idgen"
)

// Reloader applies a new runtime Config after LLM settings change.
type Reloader interface {
	Reload(cfg config.Config)
}

// Service orchestrates settings CRUD against the JSON store.
type Service struct {
	mu            sync.Mutex
	store         repository.SettingsStore
	envCfg        config.Config // immutable bootstrap snapshot
	reloader      Reloader
	modelImporter service.ProviderModelImporter
	providers     service.ProviderRegistry
}

// NewService wires store, hot-reload target, importer, and provider registry.
func NewService(store repository.SettingsStore, envCfg config.Config, reloader Reloader, modelImporter service.ProviderModelImporter, providers service.ProviderRegistry) *Service {
	if modelImporter == nil {
		modelImporter = &stubModelImporter{}
	}
	return &Service{store: store, envCfg: envCfg, reloader: reloader, modelImporter: modelImporter, providers: providers}
}

// Snapshot is GET /api/settings.
type Snapshot struct {
	Source     string                `json:"source"` // file | env
	ConfigPath string                `json:"config_path"`
	Users      []entity.SettingsUser `json:"users"`
	LLM        LLMPublic             `json:"llm"`
	Limits     LimitsPublic          `json:"limits"`
	Context    ContextPublic         `json:"context"`
	Docs       DocsPublic            `json:"docs"`
	WebSearch  WebSearchPublic       `json:"web_search"`
	MCP        MCPPublic             `json:"mcp"`
}

// LLMPublic is masked LLM settings for API (providers + globals).
type LLMPublic struct {
	Strategy           string           `json:"strategy"`
	ActiveProvider     string           `json:"active_provider"`
	Stub               bool             `json:"stub"`
	Stream             bool             `json:"stream"`
	Vision             string           `json:"vision"`
	Effort             string           `json:"effort"`
	TotalAttemptBudget int              `json:"total_attempt_budget"`
	RetryBaseDelayMS   int              `json:"retry_base_delay_ms"`
	RetryMaxDelayMS    int              `json:"retry_max_delay_ms"`
	RetryJitter        float64          `json:"retry_jitter"`
	Providers          []ProviderPublic `json:"providers"`
}

// ProviderPublic never includes raw api_key.
type ProviderPublic struct {
	Type           string                 `json:"type"`
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Prefix         string                 `json:"prefix,omitempty"`
	API            string                 `json:"api"`
	BaseURL        string                 `json:"base_url"`
	APIKeySet      bool                   `json:"api_key_set"`
	APIKeyMasked   string                 `json:"api_key_masked,omitempty"`
	APIKeyOptional bool                   `json:"api_key_optional,omitempty"`
	Enabled        bool                   `json:"enabled"`
	Models         []entity.SettingsModel `json:"models"`
	TimeoutSec     int                    `json:"timeout_sec,omitempty"`
	MaxAttempts    int                    `json:"max_attempts,omitempty"`
	Weight         int                    `json:"weight,omitempty"`
}

func (s *Service) GetSnapshot(ctx context.Context) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, source, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	rt := config.ApplySettingsFile(s.envCfg, doc)
	return Snapshot{
		Source:     source,
		ConfigPath: s.store.Path(),
		Users:      doc.Users,
		LLM:        enrichLLMPublic(publicLLM(rt, doc, s.providers), rt),
		Limits:     publicLimits(rt),
		Context:    publicContext(rt),
		Docs:       publicDocs(rt),
		WebSearch:  publicWebSearch(rt),
		MCP:        publicMCP(rt),
	}, nil
}

// ListProviderCatalog returns credential-free provider families available in
// this build.
func (s *Service) ListProviderCatalog() []entity.ProviderDefinition {
	if s.providers == nil {
		return []entity.ProviderDefinition{}
	}
	return s.providers.List()
}

func (s *Service) ListUsers(ctx context.Context) ([]entity.SettingsUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return nil, err
	}
	return append([]entity.SettingsUser(nil), doc.Users...), nil
}

type UserInput struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

func (s *Service) CreateUser(ctx context.Context, in UserInput) (entity.SettingsUser, error) {
	name := strings.TrimSpace(in.Name)
	role := normalizeRole(in.Role)
	if name == "" {
		return entity.SettingsUser{}, apperr.Validation("name required")
	}
	if role == "" {
		return entity.SettingsUser{}, apperr.Validation("role must be owner|admin|member")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return entity.SettingsUser{}, err
	}
	u := entity.SettingsUser{ID: idgen.New("usr"), Name: name, Role: role}
	doc.Users = append(doc.Users, u)
	if err := s.persistLocked(ctx, doc); err != nil {
		return entity.SettingsUser{}, err
	}
	return u, nil
}

func (s *Service) UpdateUser(ctx context.Context, id string, in UserInput) (entity.SettingsUser, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return entity.SettingsUser{}, apperr.Validation("id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return entity.SettingsUser{}, err
	}
	idx := -1
	for i, u := range doc.Users {
		if u.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return entity.SettingsUser{}, apperr.NotFound("user not found")
	}
	u := doc.Users[idx]
	if name := strings.TrimSpace(in.Name); name != "" {
		u.Name = name
	}
	if in.Role != "" {
		role := normalizeRole(in.Role)
		if role == "" {
			return entity.SettingsUser{}, apperr.Validation("role must be owner|admin|member")
		}
		if u.Role == "owner" && role != "owner" && countOwners(doc.Users) <= 1 {
			return entity.SettingsUser{}, apperr.Validation("cannot demote the last owner")
		}
		u.Role = role
	}
	doc.Users[idx] = u
	if err := s.persistLocked(ctx, doc); err != nil {
		return entity.SettingsUser{}, err
	}
	return u, nil
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return err
	}
	idx := -1
	for i, u := range doc.Users {
		if u.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return apperr.NotFound("user not found")
	}
	if doc.Users[idx].Role == "owner" && countOwners(doc.Users) <= 1 {
		return apperr.Validation("cannot delete the last owner")
	}
	doc.Users = append(doc.Users[:idx], doc.Users[idx+1:]...)
	return s.persistLocked(ctx, doc)
}

func (s *Service) ListProviders(ctx context.Context) ([]ProviderPublic, error) {
	snap, err := s.GetSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snap.LLM.Providers, nil
}

func (s *Service) GetProvider(ctx context.Context, id string) (ProviderPublic, error) {
	id = strings.ToUpper(strings.TrimSpace(id))
	list, err := s.ListProviders(ctx)
	if err != nil {
		return ProviderPublic{}, err
	}
	for _, p := range list {
		if p.ID == id {
			return p, nil
		}
	}
	return ProviderPublic{}, apperr.NotFound("provider not found")
}

// ProviderInput is create/update body.
type ProviderInput struct {
	Type        string                 `json:"type"`
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Prefix      string                 `json:"prefix"`
	API         string                 `json:"api"`
	BaseURL     string                 `json:"base_url"`
	APIKey      *string                `json:"api_key"` // nil = omit; "" on create = empty; non-empty = set
	Enabled     *bool                  `json:"enabled"`
	Models      []entity.SettingsModel `json:"models"`
	ModelID     string                 `json:"model_id"` // convenience for create form
	TimeoutSec  int                    `json:"timeout_sec"`
	MaxAttempts int                    `json:"max_attempts"`
	Weight      int                    `json:"weight"`
}

func (s *Service) CreateProvider(ctx context.Context, in ProviderInput) (ProviderPublic, error) {
	id := strings.ToUpper(strings.TrimSpace(in.ID))
	if id == "" {
		id = strings.ToUpper(strings.TrimSpace(in.Prefix))
	}
	if id == "" && strings.TrimSpace(in.Type) == "" {
		return ProviderPublic{}, apperr.Validation("id or prefix required")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = id
	}
	api := strings.ToLower(strings.TrimSpace(in.API))
	if api == "" && (s.providers == nil || strings.TrimSpace(in.Type) == "") {
		api = "responses"
	}
	if api != "" && api != "chat" && api != "responses" && api != "messages" {
		return ProviderPublic{}, apperr.Validation("api must be chat|responses|messages")
	}
	base := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if base == "" && (s.providers == nil || strings.TrimSpace(in.Type) == "") {
		return ProviderPublic{}, apperr.Validation("base_url required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return ProviderPublic{}, err
	}
	models := in.Models
	if len(models) == 0 {
		if mid := strings.TrimSpace(in.ModelID); mid != "" {
			models = []entity.SettingsModel{{ID: mid}}
		}
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	key := ""
	if in.APIKey != nil {
		key = strings.TrimSpace(*in.APIKey)
	}
	prefix := strings.TrimSpace(in.Prefix)
	if prefix == "" {
		prefix = strings.ToLower(id)
	}
	sp := entity.SettingsProvider{
		Type:        strings.ToLower(strings.TrimSpace(in.Type)),
		ID:          id,
		Name:        name,
		Prefix:      prefix,
		API:         api,
		BaseURL:     base,
		APIKey:      key,
		Enabled:     enabled,
		Models:      models,
		TimeoutSec:  in.TimeoutSec,
		MaxAttempts: in.MaxAttempts,
		Weight:      in.Weight,
	}
	if s.providers != nil {
		sp, err = s.providers.Normalize(sp)
		if err != nil {
			return ProviderPublic{}, apperr.Validation(err.Error())
		}
		id = sp.ID
	}
	for _, p := range doc.LLM.Providers {
		if strings.EqualFold(p.ID, id) {
			return ProviderPublic{}, apperr.Validation("provider id already exists")
		}
	}
	doc.LLM.Providers = append(doc.LLM.Providers, sp)
	if doc.LLM.ActiveProvider == "" {
		doc.LLM.ActiveProvider = id
	}
	if err := s.persistLocked(ctx, doc); err != nil {
		return ProviderPublic{}, err
	}
	s.reloadLocked(doc)
	return toPublic(sp, s.providers), nil
}

func (s *Service) UpdateProvider(ctx context.Context, id string, in ProviderInput) (ProviderPublic, error) {
	id = strings.ToUpper(strings.TrimSpace(id))
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return ProviderPublic{}, err
	}
	idx := -1
	for i, p := range doc.LLM.Providers {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ProviderPublic{}, apperr.NotFound("provider not found")
	}
	sp := doc.LLM.Providers[idx]
	if n := strings.TrimSpace(in.Name); n != "" {
		sp.Name = n
	}
	if p := strings.TrimSpace(in.Prefix); p != "" {
		sp.Prefix = p
	}
	if a := strings.ToLower(strings.TrimSpace(in.API)); a != "" {
		if a != "chat" && a != "responses" && a != "messages" {
			return ProviderPublic{}, apperr.Validation("api must be chat|responses|messages")
		}
		sp.API = a
	}
	if b := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/"); b != "" {
		sp.BaseURL = b
	}
	if in.APIKey != nil && strings.TrimSpace(*in.APIKey) != "" {
		sp.APIKey = strings.TrimSpace(*in.APIKey)
	}
	if in.Enabled != nil {
		sp.Enabled = *in.Enabled
	}
	if in.Models != nil {
		sp.Models = in.Models
	} else if mid := strings.TrimSpace(in.ModelID); mid != "" {
		if len(sp.Models) == 0 {
			sp.Models = []entity.SettingsModel{{ID: mid}}
		} else {
			sp.Models[0].ID = mid
		}
	}
	if in.TimeoutSec > 0 {
		sp.TimeoutSec = in.TimeoutSec
	}
	if in.MaxAttempts > 0 {
		sp.MaxAttempts = in.MaxAttempts
	}
	if in.Weight > 0 {
		sp.Weight = in.Weight
	}
	doc.LLM.Providers[idx] = sp
	if err := s.persistLocked(ctx, doc); err != nil {
		return ProviderPublic{}, err
	}
	s.reloadLocked(doc)
	return toPublic(sp, s.providers), nil
}

func (s *Service) DeleteProvider(ctx context.Context, id string) error {
	id = strings.ToUpper(strings.TrimSpace(id))
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return err
	}
	idx := -1
	for i, p := range doc.LLM.Providers {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return apperr.NotFound("provider not found")
	}
	doc.LLM.Providers = append(doc.LLM.Providers[:idx], doc.LLM.Providers[idx+1:]...)
	if strings.EqualFold(doc.LLM.ActiveProvider, id) {
		doc.LLM.ActiveProvider = ""
		if len(doc.LLM.Providers) > 0 {
			doc.LLM.ActiveProvider = doc.LLM.Providers[0].ID
		}
	}
	if err := s.persistLocked(ctx, doc); err != nil {
		return err
	}
	s.reloadLocked(doc)
	return nil
}

func (s *Service) AddModel(ctx context.Context, providerID string, model entity.SettingsModel) (ProviderPublic, error) {
	providerID = strings.ToUpper(strings.TrimSpace(providerID))
	mid := strings.TrimSpace(model.ID)
	if mid == "" {
		return ProviderPublic{}, apperr.Validation("model id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return ProviderPublic{}, err
	}
	idx := -1
	for i, p := range doc.LLM.Providers {
		if p.ID == providerID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ProviderPublic{}, apperr.NotFound("provider not found")
	}
	sp := doc.LLM.Providers[idx]
	for _, m := range sp.Models {
		if m.ID == mid {
			return ProviderPublic{}, apperr.Validation("model already listed")
		}
	}
	sp.Models = append(sp.Models, entity.SettingsModel{ID: mid, Label: strings.TrimSpace(model.Label)})
	doc.LLM.Providers[idx] = sp
	if err := s.persistLocked(ctx, doc); err != nil {
		return ProviderPublic{}, err
	}
	s.reloadLocked(doc)
	return toPublic(sp, s.providers), nil
}

func (s *Service) RemoveModel(ctx context.Context, providerID, modelID string) (ProviderPublic, error) {
	providerID = strings.ToUpper(strings.TrimSpace(providerID))
	modelID = strings.TrimSpace(modelID)
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return ProviderPublic{}, err
	}
	idx := -1
	for i, p := range doc.LLM.Providers {
		if p.ID == providerID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ProviderPublic{}, apperr.NotFound("provider not found")
	}
	sp := doc.LLM.Providers[idx]
	found := false
	next := make([]entity.SettingsModel, 0, len(sp.Models))
	for _, m := range sp.Models {
		if m.ID == modelID {
			found = true
			continue
		}
		next = append(next, m)
	}
	if !found {
		return ProviderPublic{}, apperr.NotFound("model not found")
	}
	sp.Models = next
	doc.LLM.Providers[idx] = sp
	if err := s.persistLocked(ctx, doc); err != nil {
		return ProviderPublic{}, err
	}
	s.reloadLocked(doc)
	return toPublic(sp, s.providers), nil
}

type stubModelImporter struct{}

func (stubModelImporter) ImportModels(context.Context, entity.SettingsProvider) ([]entity.SettingsModel, error) {
	return nil, apperr.NotImplemented("model import")
}

// ImportModels fetches the provider's /models endpoint and appends any new
// model ids to the provider's model list.
func (s *Service) ImportModels(ctx context.Context, providerID string) (map[string]any, error) {
	providerID = strings.ToUpper(strings.TrimSpace(providerID))
	if providerID == "" {
		return nil, apperr.Validation("provider id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, _, err := s.loadOrSeedLocked(ctx)
	if err != nil {
		return nil, err
	}
	idx := -1
	for i, p := range doc.LLM.Providers {
		if p.ID == providerID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, apperr.NotFound("provider not found")
	}
	sp := doc.LLM.Providers[idx]

	imported, err := s.modelImporter.ImportModels(ctx, sp)
	if err != nil {
		return nil, err
	}

	existing := make(map[string]int, len(sp.Models))
	for i, m := range sp.Models {
		existing[m.ID] = i
	}
	added := 0
	updated := 0
	for _, m := range imported {
		if idx, ok := existing[m.ID]; ok {
			old := sp.Models[idx]
			if metadataChanged(old, m) {
				sp.Models[idx] = m
				updated++
			}
			continue
		}
		sp.Models = append(sp.Models, m)
		existing[m.ID] = len(sp.Models) - 1
		added++
	}
	doc.LLM.Providers[idx] = sp
	if err := s.persistLocked(ctx, doc); err != nil {
		return nil, err
	}
	s.reloadLocked(doc)

	msg := fmt.Sprintf("Imported %d new model(s) from %s.", added, providerID)
	if updated > 0 {
		msg = fmt.Sprintf("Imported %d new, updated %d model(s) from %s.", added, updated, providerID)
	}
	return map[string]any{
		"imported": added,
		"updated":  updated,
		"total":    len(sp.Models),
		"message":  msg,
	}, nil
}

func metadataChanged(old, new entity.SettingsModel) bool {
	if old.Task != new.Task {
		return true
	}
	if old.ContextWindow != new.ContextWindow || old.MaxOutput != new.MaxOutput {
		return true
	}
	if old.SupportsTools != new.SupportsTools || old.Description != new.Description {
		return true
	}
	if !stringSliceEqual(old.InputModes, new.InputModes) || !stringSliceEqual(old.OutputModes, new.OutputModes) {
		return true
	}
	if !stringSliceEqual(old.EffortLevels, new.EffortLevels) {
		return true
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *Service) loadOrSeedLocked(ctx context.Context) (entity.SettingsFile, string, error) {
	if s.store.Exists() {
		doc, err := s.store.Load(ctx)
		if err != nil {
			return entity.SettingsFile{}, "", apperr.Wrap(500, apperr.CodeInternal, "load config", err)
		}
		if len(doc.Users) == 0 {
			doc.Users = []entity.SettingsUser{{ID: "usr_owner", Name: "Owner", Role: "owner"}}
		}
		src := "file"
		if len(doc.LLM.Providers) == 0 {
			src = "env"
		}
		return doc, src, nil
	}
	// In-memory seed from defaults (not written until first mutate).
	doc := config.DefaultSeedFile(s.envCfg)
	return doc, "env", nil
}

func (s *Service) persistLocked(ctx context.Context, doc entity.SettingsFile) error {
	if err := s.store.Save(ctx, doc); err != nil {
		return apperr.Wrap(500, apperr.CodeInternal, "save config", err)
	}
	return nil
}

func (s *Service) reloadLocked(doc entity.SettingsFile) {
	if s.reloader == nil {
		return
	}
	s.reloader.Reload(config.ApplySettingsFile(s.envCfg, doc))
}

func publicLLM(rt config.Config, doc entity.SettingsFile, registry service.ProviderRegistry) LLMPublic {
	out := LLMPublic{
		Strategy:       rt.LLMStrategy,
		ActiveProvider: rt.LLMActiveProvider,
		Stub:           rt.LLMStub,
		Providers:      nil,
	}
	if len(doc.LLM.Providers) > 0 {
		for _, sp := range doc.LLM.Providers {
			out.Providers = append(out.Providers, toPublic(sp, registry))
		}
		return out
	}
	// Env-sourced: synthesize public view from runtime map.
	for _, sp := range config.RuntimeProvidersToFile(rt.LLMProviders) {
		out.Providers = append(out.Providers, toPublic(sp, registry))
	}
	return out
}

func toPublic(sp entity.SettingsProvider, registry service.ProviderRegistry) ProviderPublic {
	set, masked := config.MaskAPIKey(sp.APIKey)
	models := sp.Models
	if models == nil {
		models = []entity.SettingsModel{}
	}
	kind := strings.ToLower(strings.TrimSpace(sp.Type))
	if kind == "" && registry != nil {
		kind = registry.Infer(sp)
	}
	return ProviderPublic{
		Type:           kind,
		ID:             sp.ID,
		Name:           sp.Name,
		Prefix:         sp.Prefix,
		API:            sp.API,
		BaseURL:        sp.BaseURL,
		APIKeySet:      set,
		APIKeyMasked:   masked,
		APIKeyOptional: sp.APIKeyOptional,
		Enabled:        sp.Enabled,
		Models:         models,
		TimeoutSec:     sp.TimeoutSec,
		MaxAttempts:    sp.MaxAttempts,
		Weight:         sp.Weight,
	}
}

func normalizeRole(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "owner", "admin", "member":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func countOwners(users []entity.SettingsUser) int {
	n := 0
	for _, u := range users {
		if u.Role == "owner" {
			n++
		}
	}
	return n
}
