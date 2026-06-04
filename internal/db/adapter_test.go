package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cskiller24/querylex/internal/db"
	_ "github.com/cskiller24/querylex/internal/db/mysql"
	_ "github.com/cskiller24/querylex/internal/db/postgresql"
	_ "github.com/cskiller24/querylex/internal/db/sqlite"
	_ "github.com/cskiller24/querylex/internal/db/mariadb"
	_ "github.com/cskiller24/querylex/internal/db/mssql"
)

func TestAdapterMethods_ConcreteTypes(t *testing.T) {
	// This test verifies that the adapter interface methods return concrete types
	// (*db.SchemaResult, *db.ExplainPlan, etc.) instead of `any`.
	adapter, err := db.Open("mysql", "")
	if err != nil {
		t.Fatalf("Open(mysql) failed: %v", err)
	}

	ctx := context.Background()

	// Schema, Stats, Indexes are now implemented — tested in their respective test files.
	// Validate is now implemented (returns result without connection for non-DML queries).
	// Explain and Joins are still stubs and return ErrNotImplemented without a connection.

	t.Run("Explain returns *ExplainPlan", func(t *testing.T) {
		result, err := adapter.Explain(ctx, "", false)
		if err != db.ErrNotImplemented {
			t.Fatalf("expected ErrNotImplemented, got %v", err)
		}
		if result != nil {
			t.Fatalf("expected nil result, got %v", result)
		}
		var _ *db.ExplainPlan = result
	})

	t.Run("Validate returns *ValidateResult when not connected", func(t *testing.T) {
		result, err := adapter.Validate(ctx, "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil ValidateResult")
		}
		if !result.Valid {
			t.Fatalf("expected Valid=true with empty query, got Valid=%v", result.Valid)
		}
		if !result.ReadOnly {
			t.Fatalf("expected ReadOnly=true, got ReadOnly=%v", result.ReadOnly)
		}
		var _ *db.ValidateResult = result
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
	})
}

func TestAdapterMethods_Implemented(t *testing.T) {
	adapter, err := db.Open("mysql", "")
	if err != nil {
		t.Fatalf("Open(mysql) failed: %v", err)
	}

	// Schema, Stats, Indexes are implemented.
	// Validate is now implemented (works without connection).
	// Explain and Joins are still stubs — return ErrNotImplemented.
	methods := []struct {
		name    string
		fn      func() (any, error)
		wantErr string // substring expected in error, ""=no error, "ErrNotImplemented" for stubs
	}{
		{"Explain", func() (any, error) { return adapter.Explain(context.Background(), "", false) }, "ErrNotImplemented"},
		{"Validate", func() (any, error) { return adapter.Validate(context.Background(), "") }, ""},
		{"Joins", func() (any, error) { return adapter.Joins(context.Background(), nil) }, "ErrNotImplemented"},
	}

	for _, m := range methods {
		result, err := m.fn()
		if m.wantErr == "ErrNotImplemented" {
			if err != db.ErrNotImplemented {
				t.Errorf("%s: expected ErrNotImplemented, got %v", m.name, err)
			}
		} else if m.wantErr != "" {
			if err == nil {
				t.Errorf("%s: expected error containing %q, got nil result=%v", m.name, m.wantErr, result)
				continue
			}
			if !strings.Contains(err.Error(), m.wantErr) {
				t.Errorf("%s: expected error containing %q, got %v", m.name, m.wantErr, err)
			}
		} else {
			if err != nil {
				t.Errorf("%s: expected no error, got %v", m.name, err)
			}
			if result == nil {
				t.Errorf("%s: expected non-nil result", m.name)
			}
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

	t.Run("mariadb adapter returns mariadb", func(t *testing.T) {
		adapter, err := db.Open("mariadb", "")
		if err != nil {
			t.Fatalf("Open(mariadb) failed: %v", err)
		}
		if adapter.DatabaseType() != "mariadb" {
			t.Fatalf("expected mariadb, got %s", adapter.DatabaseType())
		}
	})

	t.Run("mssql adapter returns mssql", func(t *testing.T) {
		adapter, err := db.Open("mssql", "")
		if err != nil {
			t.Fatalf("Open(mssql) failed: %v", err)
		}
		if adapter.DatabaseType() != "mssql" {
			t.Fatalf("expected mssql, got %s", adapter.DatabaseType())
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

	t.Run("sqlite registered", func(t *testing.T) {
		adapter, err := db.Open("sqlite", "")
		if err != nil {
			t.Fatalf("Open(sqlite) failed: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter for sqlite")
		}
	})

	t.Run("mariadb registered", func(t *testing.T) {
		adapter, err := db.Open("mariadb", "")
		if err != nil {
			t.Fatalf("Open(mariadb) failed: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter for mariadb")
		}
		if dt := adapter.DatabaseType(); dt != "mariadb" {
			t.Fatalf("expected DatabaseType()=mariadb, got %s", dt)
		}
	})

	t.Run("mssql registered", func(t *testing.T) {
		adapter, err := db.Open("mssql", "")
		if err != nil {
			t.Fatalf("Open(mssql) failed: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter for mssql")
		}
		if dt := adapter.DatabaseType(); dt != "mssql" {
			t.Fatalf("expected DatabaseType()=mssql, got %s", dt)
		}
	})

	t.Run("unknown type returns error", func(t *testing.T) {
		_, err := db.Open("unknown", "")
		if err == nil {
			t.Fatal("expected error for unknown database type")
		}
	})
}
