package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/enterpilot/gomodel/internal/thinkingoverrides"
)

type thinkingOverrideTestStore struct {
	items map[string]thinkingoverrides.Override
}

func newThinkingOverrideTestStore(items ...thinkingoverrides.Override) *thinkingOverrideTestStore {
	store := &thinkingOverrideTestStore{items: make(map[string]thinkingoverrides.Override, len(items))}
	for _, item := range items {
		store.items[item.Selector] = item
	}
	return store
}

func (s *thinkingOverrideTestStore) List(_ context.Context) ([]thinkingoverrides.Override, error) {
	result := make([]thinkingoverrides.Override, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, item)
	}
	return result, nil
}

func (s *thinkingOverrideTestStore) Upsert(_ context.Context, override thinkingoverrides.Override) error {
	s.items[override.Selector] = override
	return nil
}

func (s *thinkingOverrideTestStore) Delete(_ context.Context, selector string) error {
	if _, ok := s.items[selector]; !ok {
		return thinkingoverrides.ErrNotFound
	}
	delete(s.items, selector)
	return nil
}

func (s *thinkingOverrideTestStore) Close() error { return nil }

type thinkingOverrideTestCatalog struct {
	providerNames []string
}

func (c thinkingOverrideTestCatalog) ProviderNames() []string {
	return append([]string(nil), c.providerNames...)
}

func newThinkingOverrideService(t *testing.T, store thinkingoverrides.Store, providerNames ...string) *thinkingoverrides.Service {
	t.Helper()
	if len(providerNames) == 0 {
		providerNames = []string{"RunInfra", "openai"}
	}
	service, err := thinkingoverrides.NewService(store, thinkingOverrideTestCatalog{providerNames: providerNames})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	return service
}

func TestModelThinkingOverrideLifecycle(t *testing.T) {
	service := newThinkingOverrideService(t, newThinkingOverrideTestStore())
	h := NewHandler(nil, nil, WithThinkingOverrides(service))
	e := echo.New()
	h.RegisterRoutes(e.Group("/admin"))

	body := `{"selector":"RunInfra/qwen3-8-27b","mode":"override","thinking":true,"effort":"high"}`
	putReq := httptest.NewRequest(http.MethodPut, "/admin/model-thinking-overrides", bytes.NewBufferString(body))
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	e.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200 body=%s", putRec.Code, putRec.Body.String())
	}
	var saved thinkingoverrides.Override
	if err := json.Unmarshal(putRec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode upsert response: %v", err)
	}
	if saved.Selector != "RunInfra/qwen3-8-27b" || saved.Mode != thinkingoverrides.ModeOverride ||
		!saved.Thinking || saved.Effort != thinkingoverrides.EffortHigh {
		t.Fatalf("saved override = %#v", saved)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/admin/model-thinking-overrides", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listRec.Code)
	}
	var list []thinkingoverrides.Override
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(list) != 1 || list[0].Selector != "RunInfra/qwen3-8-27b" {
		t.Fatalf("list = %#v, want one RunInfra/qwen3-8-27b row", list)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/admin/model-thinking-overrides", bytes.NewBufferString(`{"selector":"RunInfra/qwen3-8-27b"}`))
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteRec := httptest.NewRecorder()
	e.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", deleteRec.Code)
	}
}

func TestUpsertModelThinkingOverrideRejectsInvalidEffort(t *testing.T) {
	service := newThinkingOverrideService(t, newThinkingOverrideTestStore())
	h := NewHandler(nil, nil, WithThinkingOverrides(service))
	e := echo.New()
	h.RegisterRoutes(e.Group("/admin"))

	body := `{"selector":"RunInfra/qwen3-8-27b","mode":"override","thinking":true,"effort":"turbo"}`
	req := httptest.NewRequest(http.MethodPut, "/admin/model-thinking-overrides", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("put status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
}
