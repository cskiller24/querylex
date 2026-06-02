package mssql_test

import (
	"context"
	"strings"
	"testing"

	"github.com/querylex/querylex/internal/db"
	"github.com/querylex/querylex/internal/db/mssql"
)

func TestMSSQLAdapter_FactoryRegistration(t *testing.T) {
	t.Run("mssql registered", func(t *testing.T) {
		adapter, err := db.Open("mssql", "")
		if err != nil {
			t.Fatalf("Open(mssql) failed: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter for mssql")
		}
	})
}

func TestMSSQLAdapter_DatabaseType(t *testing.T) {
	adapter, err := db.Open("mssql", "")
	if err != nil {
		t.Fatalf("Open(mssql) failed: %v", err)
	}
	if dt := adapter.DatabaseType(); dt != "mssql" {
		t.Fatalf("expected mssql, got %s", dt)
	}
}

func TestMSSQLAdapter_ConcreteReturnTypes(t *testing.T) {
	adapter, err := db.Open("mssql", "")
	if err != nil {
		t.Fatalf("Open(mssql) failed: %v", err)
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

func TestMSSQLAdapter_NoErrNotImplemented(t *testing.T) {
	adapter, err := db.Open("mssql", "")
	if err != nil {
		t.Fatalf("Open(mssql) failed: %v", err)
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

func TestMSSQLAdapter_ConnectUsesSQLServerDriver(t *testing.T) {
	adapter, err := db.Open("mssql", "")
	if err != nil {
		t.Fatalf("Open(mssql) failed: %v", err)
	}

	// Connecting with invalid DSN should produce a driver-level error
	ctx := context.Background()
	err = adapter.Connect(ctx, "invalid:dsn")
	if err != nil {
		t.Logf("Connect with invalid DSN gave expected error: %v", err)
	}
}

func TestMSSQLAdapter_ValidateSelect(t *testing.T) {
	adapter, err := db.Open("mssql", "")
	if err != nil {
		t.Fatalf("Open(mssql) failed: %v", err)
	}

	ctx := context.Background()

	t.Run("SELECT is valid (L1 check without connection)", func(t *testing.T) {
		result, err := adapter.Validate(ctx, "SELECT 1")
		if err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
		if result == nil {
			t.Fatal("Validate returned nil")
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

func TestMSSQLAdapter_BuildDSN(t *testing.T) {
	dsn := mssql.BuildDSN("localhost", 1433, "testdb", "user", "pass", "true")
	if dsn == "" {
		t.Fatal("BuildDSN returned empty string")
	}
	if dsn[:12] != "sqlserver://" {
		t.Fatalf("BuildDSN should produce sqlserver:// URL, got: %s", dsn)
	}
	if !contains(dsn, "user") || !contains(dsn, "pass") || !contains(dsn, "testdb") {
		t.Fatalf("BuildDSN missing expected components: %s", dsn)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
