package thinkingoverrides

import (
	"context"
	"testing"

	"github.com/enterpilot/gomodel/internal/storage/sqlx"
	"github.com/enterpilot/gomodel/internal/storage/sqlx/sqlxtest"
)

func runSQLStoreTest(t *testing.T, body func(t *testing.T, store *SQLStore)) {
	t.Helper()
	sqlxtest.Run(t, func(t *testing.T, db sqlx.DB) {
		store, err := NewSQLStore(context.Background(), db)
		if err != nil {
			t.Fatalf("NewSQLStore: %v", err)
		}
		t.Cleanup(func() { _ = store.Close() })
		body(t, store)
	})
}

func TestSQLStoreThinkingOverrideCRUD(t *testing.T) {
	runSQLStoreTest(t, func(t *testing.T, store *SQLStore) {
		ctx := context.Background()

		override := Override{
			Selector: "RunInfra/qwen3-8-27b",
			ProviderName: "RunInfra",
			Model: "qwen3-8-27b",
			Mode: ModeOverride,
			Thinking: true,
			Effort: EffortHigh,
		}
		if err := store.Upsert(ctx, override); err != nil {
			t.Fatalf("Upsert: %v", err)
		}

		list, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("len(List) = %d, want 1", len(list))
		}
		got := list[0]
		if got.Selector != "RunInfra/qwen3-8-27b" || got.Mode != ModeOverride || !got.Thinking || got.Effort != EffortHigh {
			t.Fatalf("List[0] = %#v", got)
		}

		// Upsert updates the same row.
		override.Thinking = false
		if err := store.Upsert(ctx, override); err != nil {
			t.Fatalf("Upsert update: %v", err)
		}
		list, err = store.List(ctx)
		if err != nil {
			t.Fatalf("List after update: %v", err)
		}
		if len(list) != 1 || list[0].Thinking {
			t.Fatalf("List after update = %#v, want thinking=false", list)
		}

		if err := store.Delete(ctx, "RunInfra/qwen3-8-27b"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		list, err = store.List(ctx)
		if err != nil {
			t.Fatalf("List after delete: %v", err)
		}
		if len(list) != 0 {
			t.Fatalf("len(List) after delete = %d, want 0", len(list))
		}
		if err := store.Delete(ctx, "RunInfra/qwen3-8-27b"); err != ErrNotFound {
			t.Fatalf("Delete missing = %v, want ErrNotFound", err)
		}
	})
}

func TestSQLStoreThinkingOverrideRejectsInvalid(t *testing.T) {
	runSQLStoreTest(t, func(t *testing.T, store *SQLStore) {
		ctx := context.Background()
		err := store.Upsert(ctx, Override{
			Selector: "RunInfra/qwen3-8-27b",
			Mode: ModeOverride,
			Thinking: true,
			Effort: "extreme",
		})
		if err == nil {
			t.Fatal("Upsert with invalid effort succeeded, want error")
		}
	})
}
