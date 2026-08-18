package thinkingoverrides

import (
	"context"
	"errors"
)

// ErrNotFound reports a missing override during deletion.
var ErrNotFound = errors.New("model thinking override not found")

// Store persists model thinking overrides.
type Store interface {
	List(ctx context.Context) ([]Override, error)
	Upsert(ctx context.Context, override Override) error
	Delete(ctx context.Context, selector string) error
	Close() error
}
