package settings

import (
	"context"
	"path/filepath"
	"testing"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/infrastructure/provider"
	"buatpostingan/internal/infrastructure/provider/omniroute"
	"buatpostingan/internal/infrastructure/repository/appconfig"
	"buatpostingan/internal/pkg/apperr"
)

type fakeModelImporter struct {
	models []entity.SettingsModel
	err    error
}

func TestCreateTypedProviderUsesInjectedDefaults(t *testing.T) {
	dir := t.TempDir()
	store := appconfig.NewStore(filepath.Join(dir, "config.json"))
	registry, err := provider.NewRegistry(omniroute.New())
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(store, config.Config{LLMStrategy: "failover"}, nil, nil, registry)
	created, err := svc.CreateProvider(context.Background(), ProviderInput{Type: "omniroute"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Type != "omniroute" || created.ID != "OMNIROUTE" {
		t.Fatalf("identity=%+v", created)
	}
	if created.API != "responses" || created.BaseURL != "http://127.0.0.1:20128/v1" || !created.APIKeyOptional {
		t.Fatalf("defaults=%+v", created)
	}
}

func (f *fakeModelImporter) ImportModels(_ context.Context, _ entity.SettingsProvider) ([]entity.SettingsModel, error) {
	return f.models, f.err
}

func TestMetadataChangedIncludesModelTask(t *testing.T) {
	t.Parallel()
	old := entity.SettingsModel{ID: "opaque-model"}
	next := entity.SettingsModel{ID: "opaque-model", Task: "embedding"}
	if !metadataChanged(old, next) {
		t.Fatal("task metadata change must refresh the stored model")
	}
}

type fakeReloader struct {
	n    int
	last config.Config
}

func (f *fakeReloader) Reload(cfg config.Config) {
	f.n++
	f.last = cfg
}

func TestCRUDUsersAndMask(t *testing.T) {
	dir := t.TempDir()
	store := appconfig.NewStore(filepath.Join(dir, "config.json"))
	env := config.Config{
		LLMStrategy: "failover",
		LLMProviders: map[string]config.LLMProvider{
			"ENV": {ID: "ENV", APIKey: "env-secret-key", Model: "m1", Enabled: true, API: "responses"},
		},
	}
	rel := &fakeReloader{}
	svc := NewService(store, env, rel, nil, nil)
	ctx := context.Background()

	snap, err := svc.GetSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != "env" {
		t.Fatalf("source=%s", snap.Source)
	}
	if len(snap.LLM.Providers) != 1 || !snap.LLM.Providers[0].APIKeySet {
		t.Fatalf("providers: %+v", snap.LLM.Providers)
	}
	if snap.LLM.Providers[0].APIKeyMasked == "env-secret-key" {
		t.Fatal("raw key leaked")
	}

	u, err := svc.CreateUser(ctx, UserInput{Name: "Alice", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == "" || u.Name != "Alice" {
		t.Fatalf("%+v", u)
	}
	if !store.Exists() {
		t.Fatal("create user should persist file")
	}

	key := "sk-live-abcdef12"
	pub, err := svc.CreateProvider(ctx, ProviderInput{
		ID: "LOCAL", Name: "Local", API: "responses",
		BaseURL: "http://127.0.0.1/v1", APIKey: &key, ModelID: "mimo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pub.APIKeySet != true || pub.APIKeyMasked == key {
		t.Fatalf("mask fail: %+v", pub)
	}
	if rel.n < 1 {
		t.Fatal("expected reload")
	}
	if rel.last.LLMProviders["LOCAL"].APIKey != key {
		t.Fatal("reload should see raw key internally")
	}

	empty := ""
	_, err = svc.UpdateProvider(ctx, "LOCAL", ProviderInput{APIKey: &empty, Name: "Local 2"})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := svc.GetProvider(ctx, "LOCAL")
	if got.Name != "Local 2" || !got.APIKeySet {
		t.Fatalf("empty api_key should keep secret: %+v", got)
	}

	_, err = svc.AddModel(ctx, "LOCAL", entity.SettingsModel{ID: "other"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.RemoveModel(ctx, "LOCAL", "other")
	if err != nil {
		t.Fatal(err)
	}

	fakeImporter := &fakeModelImporter{models: []entity.SettingsModel{
		{ID: "m1", Label: "Model 1"},
		{ID: "m2", Label: "Model 2"},
		{ID: "m1", Label: "Model 1"}, // duplicate should be skipped
	}}
	svcWithImporter := NewService(store, env, rel, fakeImporter, nil)
	_, err = svcWithImporter.CreateProvider(ctx, ProviderInput{
		ID: "REMOTE", Name: "Remote", API: "chat",
		BaseURL: "http://example.com/v1", APIKey: &key,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svcWithImporter.ImportModels(ctx, "REMOTE")
	if err != nil {
		t.Fatal(err)
	}
	if res["imported"] != 2 {
		t.Fatalf("expected 2 imported, got %v", res["imported"])
	}
	pub, _ = svcWithImporter.GetProvider(ctx, "REMOTE")
	if len(pub.Models) != 2 {
		t.Fatalf("expected 2 models on provider, got %+v", pub.Models)
	}

	err = svc.DeleteUser(ctx, "usr_owner")
	if err == nil {
		t.Fatal("should forbid deleting last owner")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeValidation {
		t.Fatalf("want validation, got %v", err)
	}
}
