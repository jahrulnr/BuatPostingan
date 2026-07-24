package settings

import (
	"context"
	"path/filepath"
	"testing"

	"buatpostingan/internal/config"
	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/infrastructure/repository/appconfig"
	"buatpostingan/internal/pkg/apperr"
)

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
	svc := NewService(store, env, rel)
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

	err = svc.DeleteUser(ctx, "usr_owner")
	if err == nil {
		t.Fatal("should forbid deleting last owner")
	}
	if ae, ok := apperr.As(err); !ok || ae.Code != apperr.CodeValidation {
		t.Fatalf("want validation, got %v", err)
	}
}
