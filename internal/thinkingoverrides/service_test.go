package thinkingoverrides

import (
	"context"
	"testing"
)

type memoryStore struct {
	rows map[string]Override
}

func newMemoryStore() *memoryStore {
	return &memoryStore{rows: map[string]Override{}}
}

func (s *memoryStore) List(ctx context.Context) ([]Override, error) {
	result := make([]Override, 0, len(s.rows))
	for _, o := range s.rows {
		result = append(result, cloneOverride(o))
	}
	return result, nil
}

func (s *memoryStore) Upsert(ctx context.Context, override Override) error {
	s.rows[override.Selector] = override
	return nil
}

func (s *memoryStore) Delete(ctx context.Context, selector string) error {
	delete(s.rows, selector)
	return nil
}

func (s *memoryStore) Close() error { return nil }

type catalogStub struct{}

func (catalogStub) ProviderNames() []string {
	return []string{"RunInfra", "openai"}
}

func mustService(t *testing.T, rows ...Override) *Service {
	t.Helper()
	store := newMemoryStore()
	for _, o := range rows {
		if err := store.Upsert(context.Background(), o); err != nil {
			t.Fatalf("seed upsert: %v", err)
		}
	}
	service, err := NewService(store, catalogStub{})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return service
}

func TestServiceResolveScopes(t *testing.T) {
	service := mustService(t,
		Override{Selector: "RunInfra/qwen3-8-27b", ProviderName: "RunInfra", Model: "qwen3-8-27b", Mode: ModeOverride, Thinking: true, Effort: EffortHigh},
		Override{Selector: "openai/gpt-4o-mini", ProviderName: "openai", Model: "gpt-4o-mini", Mode: ModeSupplement, Thinking: true, Effort: EffortLow},
		Override{Selector: "gemma4", Mode: ModeOverride, Thinking: false},
		Override{Selector: "nvidia/", ProviderName: "nvidia", Mode: ModeOverride, Thinking: true, Effort: EffortXHigh},
	)

	cases := []struct {
		name     string
		selector string
		want     bool
		effort   string
		mode     Mode
	}{
		{"exact", "RunInfra/qwen3-8-27b", true, EffortHigh, ModeOverride},
		{"model-only fallback", "someprovider/gemma4", true, "", ModeOverride},
		{"model-only exact", "gemma4", true, "", ModeOverride},
		{"provider-only fallback", "nvidia/llama-4", true, EffortXHigh, ModeOverride},
		{"no match", "openai/gpt-5", false, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			override, ok := service.Resolve(tc.selector)
			if ok != tc.want {
				t.Fatalf("Resolve(%q) ok = %v, want %v", tc.selector, ok, tc.want)
			}
			if !ok {
				return
			}
			if override.Mode != tc.mode || override.Effort != tc.effort {
				t.Fatalf("Resolve(%q) = %#v, want mode=%s effort=%s", tc.selector, override, tc.mode, tc.effort)
			}
		})
	}
}

func TestServiceUpsertAndDelete(t *testing.T) {
	store := newMemoryStore()
	service, err := NewService(store, catalogStub{})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := service.Upsert(context.Background(), Override{
		Selector: "RunInfra/qwen3-8-27b",
		Mode:     ModeSupplement,
		Thinking: true,
		Effort:   EffortMedium,
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	override, ok := service.Resolve("RunInfra/qwen3-8-27b")
	if !ok {
		t.Fatal("Resolve after upsert: not found")
	}
	if override.Mode != ModeSupplement || override.Thinking != true || override.Effort != EffortMedium {
		t.Fatalf("Resolve = %#v", override)
	}
	if override.ProviderName != "RunInfra" || override.Model != "qwen3-8-27b" {
		t.Fatalf("normalized parts = %q/%q", override.ProviderName, override.Model)
	}

	if err := service.Delete(context.Background(), "RunInfra/qwen3-8-27b"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := service.Resolve("RunInfra/qwen3-8-27b"); ok {
		t.Fatal("Resolve after delete: still found")
	}
}

func TestServiceUpsertValidation(t *testing.T) {
	store := newMemoryStore()
	service, err := NewService(store, catalogStub{})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	err = service.Upsert(context.Background(), Override{
		Selector: "RunInfra/qwen3-8-27b",
		Mode:     ModeOverride,
		Thinking: true,
		Effort:   "turbo",
	})
	if err == nil {
		t.Fatal("Upsert with invalid effort succeeded, want error")
	}
	if !IsValidationError(err) {
		t.Fatalf("err = %v, want validation error", err)
	}
}
