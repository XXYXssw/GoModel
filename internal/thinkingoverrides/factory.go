package thinkingoverrides

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/enterpilot/gomodel/config"
	"github.com/enterpilot/gomodel/internal/storage"
	"github.com/enterpilot/gomodel/internal/storage/sqlx"
)

// Result holds the initialized thinking override service and any owned resources.
type Result struct {
	Service *Service
	Store   Store

	stopRefresh func()
	closeOnce   sync.Once
	closeErr    error
}

// Close releases resources held by the thinking override subsystem.
func (r *Result) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.stopRefresh != nil {
			r.stopRefresh()
			r.stopRefresh = nil
		}
		if r.Store != nil {
			if err := r.Store.Close(); err != nil {
				r.closeErr = fmt.Errorf("store close: %w", err)
			}
		}
	})
	return r.closeErr
}

// New creates a thinking override subsystem using an existing storage connection.
func New(ctx context.Context, cfg *config.Config, shared storage.Storage, catalog Catalog) (*Result, error) {
	if shared == nil {
		return nil, fmt.Errorf("shared storage is required")
	}
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	store, err := createStore(ctx, shared)
	if err != nil {
		return nil, err
	}
	service, err := NewService(store, catalog)
	if err != nil {
		return nil, err
	}
	if err := service.Refresh(ctx); err != nil {
		return nil, err
	}

	refreshInterval := time.Minute
	if cfg.Workflows.RefreshInterval > 0 {
		refreshInterval = cfg.Workflows.RefreshInterval
	}
	return &Result{
		Service:     service,
		Store:       store,
		stopRefresh: service.StartBackgroundRefresh(refreshInterval),
	}, nil
}

func createStore(ctx context.Context, store storage.Storage) (Store, error) {
	return storage.ResolveSQLBackend[Store](
		ctx,
		store,
		func(db sqlx.DB) (Store, error) { return NewSQLStore(ctx, db) },
		func(db *mongo.Database) (Store, error) { return NewMongoDBStore(db) },
	)
}
