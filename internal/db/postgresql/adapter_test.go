package postgresql_test

import (
	"context"
	"testing"

	"github.com/cskiller24/querylex/internal/db"
	_ "github.com/cskiller24/querylex/internal/db/postgresql"
)

func TestPostgreSQLAdapter_FactoryRegistration(t *testing.T) {
	t.Run("postgres registered", func(t *testing.T) {
		adapter, err := db.Open("postgres", "")
		if err != nil {
			t.Fatalf("Open(postgres) failed: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter for postgres")
		}
	})

	t.Run("postgresql alias registered", func(t *testing.T) {
		adapter, err := db.Open("postgresql", "")
		if err != nil {
			t.Fatalf("Open(postgresql) failed: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter for postgresql")
		}
	})
}

func TestPostgreSQLAdapter_DatabaseType(t *testing.T) {
	adapter, err := db.Open("postgres", "")
	if err != nil {
		t.Fatalf("Open(postgres) failed: %v", err)
	}
	if dt := adapter.DatabaseType(); dt != "postgresql" {
		t.Fatalf("expected postgresql, got %s", dt)
	}
}

func TestPostgreSQLAdapter_MethodsReturnConcreteTypes(t *testing.T) {
	// These compile-time checks verify the interface returns concrete pointer types.
	// Must hold even when methods are fully implemented (never `any`).
	adapter, err := db.Open("postgres", "")
	if err != nil {
		t.Fatalf("Open(postgres) failed: %v", err)
	}
	ctx := context.Background()

	sResult, _ := adapter.Schema(ctx, nil)
	var _ *db.SchemaResult = sResult

	eResult, _ := adapter.Explain(ctx, "", false)
	var _ *db.ExplainPlan = eResult

	vResult, _ := adapter.Validate(ctx, "")
	var _ *db.ValidateResult = vResult

	stResult, _ := adapter.Stats(ctx, nil)
	var _ *db.StatsResult = stResult

	iResult, _ := adapter.Indexes(ctx, nil)
	var _ *db.IndexesResult = iResult

	jResult, _ := adapter.Joins(ctx, nil)
	var _ *db.JoinsResult = jResult

	_ = sResult
	_ = eResult
	_ = vResult
	_ = stResult
	_ = iResult
	_ = jResult
}

func TestPostgreSQLAdapter_NoErrNotImplemented(t *testing.T) {
	// CRITICAL: After implementation, no method should return ErrNotImplemented.
	// Even without a connection, they should return connection errors, not stub errors.
	adapter, err := db.Open("postgres", "")
	if err != nil {
		t.Fatalf("Open(postgres) failed: %v", err)
	}
	ctx := context.Background()

	t.Run("Schema does not return ErrNotImplemented", func(t *testing.T) {
		_, err := adapter.Schema(ctx, nil)
		if err == db.ErrNotImplemented {
			t.Fatal("Schema returned ErrNotImplemented — should be implemented")
		}
	})

	t.Run("Explain does not return ErrNotImplemented", func(t *testing.T) {
		_, err := adapter.Explain(ctx, "", false)
		if err == db.ErrNotImplemented {
			t.Fatal("Explain returned ErrNotImplemented — should be implemented")
		}
	})

	t.Run("Validate does not return ErrNotImplemented", func(t *testing.T) {
		_, err := adapter.Validate(ctx, "")
		if err == db.ErrNotImplemented {
			t.Fatal("Validate returned ErrNotImplemented — should be implemented")
		}
	})

	t.Run("Stats does not return ErrNotImplemented", func(t *testing.T) {
		_, err := adapter.Stats(ctx, nil)
		if err == db.ErrNotImplemented {
			t.Fatal("Stats returned ErrNotImplemented — should be implemented")
		}
	})

	t.Run("Indexes does not return ErrNotImplemented", func(t *testing.T) {
		_, err := adapter.Indexes(ctx, nil)
		if err == db.ErrNotImplemented {
			t.Fatal("Indexes returned ErrNotImplemented — should be implemented")
		}
	})

	t.Run("Joins does not return ErrNotImplemented", func(t *testing.T) {
		_, err := adapter.Joins(ctx, nil)
		if err == db.ErrNotImplemented {
			t.Fatal("Joins returned ErrNotImplemented — should be implemented")
		}
	})
}
