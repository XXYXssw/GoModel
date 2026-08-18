package thinkingoverrides

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/enterpilot/gomodel/internal/modelselectors"
	"github.com/enterpilot/gomodel/internal/validation"
)

// refreshTimeout bounds a single refresh read and the post-write refresh that
// keeps the in-memory snapshot aligned with persisted state.
const refreshTimeout = 30 * time.Second

// Catalog is the minimal configured-provider surface needed for selector validation.
type Catalog = modelselectors.Catalog

// Service keeps thinking overrides cached in memory and resolves effective
// settings per model selector at dispatch time.
type Service struct {
	store     Store
	catalog   Catalog
	current   atomic.Value
	refreshMu sync.Mutex
}

type snapshot struct {
	order      []string
	bySelector map[string]Override
}

func emptySnapshot() snapshot {
	return snapshot{order: []string{}, bySelector: map[string]Override{}}
}

// NewService creates a thinking override service backed by storage.
func NewService(store Store, catalog Catalog) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	service := &Service{store: store, catalog: catalog}
	service.current.Store(emptySnapshot())
	return service, nil
}

func (s *Service) snapshot() snapshot {
	if s == nil {
		return emptySnapshot()
	}
	return s.current.Load().(snapshot)
}

// Refresh reloads overrides from storage and atomically swaps the snapshot.
func (s *Service) Refresh(ctx context.Context) error {
	overrides, err := s.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list model thinking overrides: %w", err)
	}
	next, err := s.buildSnapshot(overrides)
	if err != nil {
		return err
	}
	s.current.Store(next)
	return nil
}

func (s *Service) buildSnapshot(overrides []Override) (snapshot, error) {
	next := emptySnapshot()
	next.bySelector = make(map[string]Override, len(overrides))
	seen := make(map[string]struct{}, len(overrides))
	for _, override := range overrides {
		normalized, err := normalizeStoredOverride(override)
		if err != nil {
			return snapshot{}, fmt.Errorf("load model thinking override %q: %w", override.Selector, err)
		}
		if err := normalized.Validate(); err != nil {
			return snapshot{}, fmt.Errorf("load model thinking override %q: %w", override.Selector, err)
		}
		if _, dup := seen[normalized.Selector]; dup {
			return snapshot{}, fmt.Errorf("load model thinking overrides: duplicate selector %q", normalized.Selector)
		}
		seen[normalized.Selector] = struct{}{}
		next.bySelector[normalized.Selector] = normalized
		next.order = append(next.order, normalized.Selector)
	}
	sort.Strings(next.order)
	return next, nil
}

// List returns all cached overrides sorted by selector.
func (s *Service) List() []Override {
	snap := s.snapshot()
	result := make([]Override, 0, len(snap.order))
	for _, selector := range snap.order {
		result = append(result, cloneOverride(snap.bySelector[selector]))
	}
	return result
}

// Get returns one cached override by selector.
func (s *Service) Get(selector string) (Override, bool) {
	override, ok := s.snapshot().bySelector[strings.TrimSpace(selector)]
	if !ok {
		return Override{}, false
	}
	return cloneOverride(override), true
}

// Resolve returns the override that applies to a qualified model selector
// (e.g. "RunInfra/qwen3-8-27b"). Exact provider/model rows win, then a
// model-only row ("qwen3-8-27b"), then a provider-only row ("RunInfra/").
func (s *Service) Resolve(selector string) (Override, bool) {
	snap := s.snapshot()
	selector = strings.TrimSpace(selector)
	if override, ok := snap.bySelector[selector]; ok {
		return cloneOverride(override), true
	}
	provider, model, split := strings.Cut(selector, "/")
	if split && model != "" {
		if override, ok := snap.bySelector[model]; ok {
			return cloneOverride(override), true
		}
	}
	if split && provider != "" {
		if override, ok := snap.bySelector[provider+"/"]; ok {
			return cloneOverride(override), true
		}
	}
	return Override{}, false
}

// Upsert validates and stores one override, then refreshes the snapshot.
func (s *Service) Upsert(ctx context.Context, override Override) error {
	if s == nil {
		return fmt.Errorf("model thinking override service is required")
	}
	normalized, err := normalizeOverrideInput(s.catalog, override)
	if err != nil {
		return err
	}
	if err := normalized.Validate(); err != nil {
		return err
	}
	if err := s.store.Upsert(ctx, normalized); err != nil {
		return fmt.Errorf("upsert model thinking override: %w", err)
	}
	s.refreshBestEffort()
	return nil
}

// Delete removes one override and refreshes the snapshot.
func (s *Service) Delete(ctx context.Context, selector string) error {
	if s == nil {
		return fmt.Errorf("model thinking override service is required")
	}
	if err := s.store.Delete(ctx, strings.TrimSpace(selector)); err != nil {
		return err
	}
	s.refreshBestEffort()
	return nil
}

// refreshBestEffort re-reads the store with a background timeout so the
// caller's request context cannot cancel the refresh. A failure only delays
// the snapshot; the next background refresh reconciles it.
func (s *Service) refreshBestEffort() {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	if err := s.Refresh(ctx); err != nil {
		slog.Warn("model thinking override snapshot refresh failed", "error", err)
	}
}

// StartBackgroundRefresh periodically reloads overrides from storage until
// the returned stop function is called.
func (s *Service) StartBackgroundRefresh(interval time.Duration) func() {
	interval = normalizedRefreshInterval(interval)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshCtx, refreshCancel := context.WithTimeout(ctx, refreshTimeout)
				if err := s.Refresh(refreshCtx); err != nil {
					slog.Error("failed to refresh model thinking overrides", "error", err)
				}
				refreshCancel()
			}
		}
	}()

	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

func normalizedRefreshInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return time.Minute
	}
	if interval < refreshTimeout {
		return refreshTimeout
	}
	return interval
}

func normalizeOverrideInput(catalog Catalog, override Override) (Override, error) {
	parts, err := modelselectors.NormalizeInput(catalog, override.Selector)
	if err != nil {
		return Override{}, err
	}
	override.Selector = parts.Selector
	override.ProviderName = parts.ProviderName
	override.Model = parts.Model

	now := time.Now().UTC()
	if override.CreatedAt.IsZero() {
		override.CreatedAt = now
	}
	override.UpdatedAt = now
	return override, nil
}

func normalizeStoredOverride(override Override) (Override, error) {
	parts, err := modelselectors.NormalizeStored(override.Selector, override.ProviderName, override.Model)
	if err != nil {
		return Override{}, err
	}
	override.Selector = parts.Selector
	override.ProviderName = parts.ProviderName
	override.Model = parts.Model
	return override, nil
}

// IsValidationError reports whether err is a selector or override validation error.
func IsValidationError(err error) bool {
	return modelselectors.IsValidationError(err) || validation.IsError(err)
}
