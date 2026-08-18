package thinkingoverrides

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enterpilot/gomodel/internal/storage/sqlx"
)

// SQLStore stores model thinking overrides in a SQL database.
type SQLStore struct {
	db sqlx.DB
}

var sqlSchema = []string{
	`CREATE TABLE IF NOT EXISTS model_thinking_overrides (
		selector TEXT PRIMARY KEY,
		provider_name TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		mode TEXT NOT NULL DEFAULT 'native',
		thinking INTEGER NOT NULL DEFAULT TRUE,
		effort TEXT NOT NULL DEFAULT '',
		created_at ` + sqlx.TypeInt64 + ` NOT NULL,
		updated_at ` + sqlx.TypeInt64 + ` NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_model_thinking_overrides_provider_name ON model_thinking_overrides(provider_name)`,
	`CREATE INDEX IF NOT EXISTS idx_model_thinking_overrides_model ON model_thinking_overrides(model)`,
	`CREATE INDEX IF NOT EXISTS idx_model_thinking_overrides_updated_at ON model_thinking_overrides(updated_at DESC)`,
}

// NewSQLStore creates the model_thinking_overrides table and indexes if needed.
func NewSQLStore(ctx context.Context, db sqlx.DB) (*SQLStore, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if err := db.Schema(ctx, sqlSchema...); err != nil {
		return nil, fmt.Errorf("failed to create model_thinking_overrides table: %w", err)
	}
	return &SQLStore{db: db}, nil
}

func (s *SQLStore) List(ctx context.Context) ([]Override, error) {
	rows, err := s.db.Query(ctx, `
		SELECT selector, provider_name, model, mode, thinking, effort, created_at, updated_at
		FROM model_thinking_overrides
		ORDER BY selector ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list model thinking overrides: %w", err)
	}
	defer rows.Close()

	result := make([]Override, 0, 8)
	for rows.Next() {
		override, err := scanSQLOverride(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, override)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model thinking overrides: %w", err)
	}
	return result, nil
}

func (s *SQLStore) Upsert(ctx context.Context, override Override) error {
	if err := override.Validate(); err != nil {
		return err
	}
	thinking := 0
	if override.Thinking {
		thinking = 1
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO model_thinking_overrides (
			selector, provider_name, model, mode, thinking, effort, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(selector) DO UPDATE SET
			provider_name = excluded.provider_name,
			model = excluded.model,
			mode = excluded.mode,
			thinking = excluded.thinking,
			effort = excluded.effort,
			updated_at = excluded.updated_at
	`,
		override.Selector,
		override.ProviderName,
		override.Model,
		string(override.Mode),
		thinking,
		override.Effort,
		override.CreatedAt.Unix(),
		override.UpdatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("upsert model thinking override: %w", err)
	}
	return nil
}

func (s *SQLStore) Delete(ctx context.Context, selector string) error {
	affected, err := s.db.Exec(ctx,
		`DELETE FROM model_thinking_overrides WHERE selector = ?`, strings.TrimSpace(selector))
	if err != nil {
		return fmt.Errorf("delete model thinking override: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLStore) Close() error {
	return nil
}

func scanSQLOverride(rows sqlx.Rows) (Override, error) {
	var override Override
	var mode string
	var thinking int
	var createdAt int64
	var updatedAt int64
	if err := rows.Scan(
		&override.Selector,
		&override.ProviderName,
		&override.Model,
		&mode,
		&thinking,
		&override.Effort,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Override{}, fmt.Errorf("scan model thinking override: %w", err)
	}
	override.Mode = Mode(mode)
	override.Thinking = thinking != 0
	override.CreatedAt = time.Unix(createdAt, 0).UTC()
	override.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return override, nil
}
