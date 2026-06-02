package sqlite_test

import (
	"context"
	"testing"

	"github.com/cskiller24/querylex/internal/db"
	_ "github.com/cskiller24/querylex/internal/db/sqlite"
)

func TestSQLiteAdapter_FactoryRegistration(t *testing.T) {
	t.Run("sqlite registered", func(t *testing.T) {
		adapter, err := db.Open("sqlite", "")
		if err != nil {
			t.Fatalf("Open(sqlite) failed: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter for sqlite")
		}
	})
}

func TestSQLiteAdapter_DatabaseType(t *testing.T) {
	adapter, err := db.Open("sqlite", "")
	if err != nil {
		t.Fatalf("Open(sqlite) failed: %v", err)
	}
	if dt := adapter.DatabaseType(); dt != "sqlite" {
		t.Fatalf("expected sqlite, got %s", dt)
	}
}

func TestSQLiteAdapter_ConcreteReturnTypes(t *testing.T) {
	adapter, err := db.Open("sqlite", "")
	if err != nil {
		t.Fatalf("Open(sqlite) failed: %v", err)
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

func TestSQLiteAdapter_NoErrNotImplemented(t *testing.T) {
	adapter, err := db.Open("sqlite", "")
	if err != nil {
		t.Fatalf("Open(sqlite) failed: %v", err)
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

func TestSQLiteAdapter_ConnectAndPing(t *testing.T) {
	adapter, err := db.Open("sqlite", "")
	if err != nil {
		t.Fatalf("Open(sqlite) failed: %v", err)
	}

	ctx := context.Background()

	// Connect to in-memory database
	if err := adapter.Connect(ctx, ":memory:"); err != nil {
		t.Fatalf("Connect(:memory:) failed: %v", err)
	}
	defer adapter.Close(ctx)

	// Ping
	if err := adapter.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestSQLiteAdapter_ValidateSelect(t *testing.T) {
	adapter, err := db.Open("sqlite", "")
	if err != nil {
		t.Fatalf("Open(sqlite) failed: %v", err)
	}

	ctx := context.Background()
	if err := adapter.Connect(ctx, ":memory:"); err != nil {
		t.Fatalf("Connect(:memory:) failed: %v", err)
	}
	defer adapter.Close(ctx)

	t.Run("SELECT is valid", func(t *testing.T) {
		result, err := adapter.Validate(ctx, "SELECT 1")
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
		if result == nil {
			t.Fatal("Validate returned nil")
		}
		if !result.Valid {
			t.Fatalf("expected valid SELECT, got errors: %v", result.Errors)
		}
	})

	t.Run("INSERT is rejected (DML)", func(t *testing.T) {
		result, err := adapter.Validate(ctx, "INSERT INTO test VALUES(1)")
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
		if result == nil {
			t.Fatal("Validate returned nil")
		}
		if result.Valid {
			t.Fatal("expected INSERT to be invalid (DML)")
		}
	})
}

func TestSQLiteAdapter_SchemaOnEmptyDB(t *testing.T) {
	adapter, err := db.Open("sqlite", "")
	if err != nil {
		t.Fatalf("Open(sqlite) failed: %v", err)
	}

	ctx := context.Background()
	if err := adapter.Connect(ctx, ":memory:"); err != nil {
		t.Fatalf("Connect(:memory:) failed: %v", err)
	}
	defer adapter.Close(ctx)

	result, err := adapter.Schema(ctx, nil)
	if err != nil {
		t.Fatalf("Schema on empty DB failed: %v", err)
	}
	if result == nil {
		t.Fatal("Schema on empty DB returned nil")
	}
	// Empty DB is valid — expect zero tables, not an error
}
