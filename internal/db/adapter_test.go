package db_test

import (
	"context"
	"testing"

	"github.com/querylex/querylex/internal/db"
	_ "github.com/querylex/querylex/internal/db/mysql"
	_ "github.com/querylex/querylex/internal/db/postgresql"
)

func TestAdapterStubs_ConcreteTypes(t *testing.T) {
	// This test verifies that the adapter interface methods return concrete types
	// (*db.SchemaResult, *db.ExplainPlan, etc.) instead of `any`.
	adapter, err := db.Open("mysql", "")
	if err != nil {
		t.Fatalf("Open(mysql) failed: %v", err)
	}

	ctx := context.Background()

	// Schema, Stats, Indexes are now implemented — tested in their respective test files.
	// Remaining 3 methods should still return ErrNotImplemented as stubs.

	t.Run("Explain returns *ExplainPlan", func(t *testing.T) {
		result, err := adapter.Explain(ctx, "", false)
		if err != db.ErrNotImplemented {
			t.Fatalf("expected ErrNotImplemented, got %v", err)
		}
		if result != nil {
			t.Fatalf("expected nil result, got %v", result)
		}
		var _ *db.ExplainPlan = result
		_ = result
	})

	t.Run("Validate returns *ValidateResult", func(t *testing.T) {
		result, err := adapter.Validate(ctx, "")
		if err != db.ErrNotImplemented {
			t.Fatalf("expected ErrNotImplemented, got %v", err)
		}
		if result != nil {
			t.Fatalf("expected nil result, got %v", result)
		}
		var _ *db.ValidateResult = result
		_ = result
	})

	t.Run("Joins returns *JoinsResult", func(t *testing.T) {
		result, err := adapter.Joins(ctx, nil)
		if err != db.ErrNotImplemented {
			t.Fatalf("expected ErrNotImplemented, got %v", err)
		}
		if result != nil {
			t.Fatalf("expected nil result, got %v", result)
		}
		var _ *db.JoinsResult = result
		_ = result
	})
}

func TestAdapterStubs_ErrNotImplemented(t *testing.T) {
	adapter, err := db.Open("mysql", "")
	if err != nil {
		t.Fatalf("Open(mysql) failed: %v", err)
	}

	// Schema, Stats, Indexes are now implemented — not checked here.
	// Remaining 3 methods should still return ErrNotImplemented as stubs.
	stubMethods := []struct {
		name string
		fn   func() (any, error)
	}{
		{"Explain", func() (any, error) { return adapter.Explain(context.Background(), "", false) }},
		{"Validate", func() (any, error) { return adapter.Validate(context.Background(), "") }},
		{"Joins", func() (any, error) { return adapter.Joins(context.Background(), nil) }},
	}

	for _, m := range stubMethods {
		result, err := m.fn()
		if err == nil {
			t.Errorf("%s: expected ErrNotImplemented, got nil result=%v", m.name, result)
			continue
		}
		if err != db.ErrNotImplemented {
			t.Errorf("%s: expected ErrNotImplemented, got %v", m.name, err)
		}
	}
}

func TestDatabaseType(t *testing.T) {
	t.Run("mysql adapter returns mysql", func(t *testing.T) {
		adapter, err := db.Open("mysql", "")
		if err != nil {
			t.Fatalf("Open(mysql) failed: %v", err)
		}
		if adapter.DatabaseType() != "mysql" {
			t.Fatalf("expected mysql, got %s", adapter.DatabaseType())
		}
	})

	t.Run("postgres adapter returns postgresql", func(t *testing.T) {
		adapter, err := db.Open("postgres", "")
		if err != nil {
			t.Fatalf("Open(postgres) failed: %v", err)
		}
		// PostgreSQL adapter's DatabaseType() returns "postgresql"
		if adapter.DatabaseType() != "postgresql" {
			t.Fatalf("expected postgresql, got %s", adapter.DatabaseType())
		}
	})
}

func TestFactoryRegistration(t *testing.T) {
	t.Run("mysql registered", func(t *testing.T) {
		adapter, err := db.Open("mysql", "")
		if err != nil {
			t.Fatalf("Open(mysql) failed: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter for mysql")
		}
	})

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

	t.Run("unknown type returns error", func(t *testing.T) {
		_, err := db.Open("unknown", "")
		if err == nil {
			t.Fatal("expected error for unknown database type")
		}
	})
}
