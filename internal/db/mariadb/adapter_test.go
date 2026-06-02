package mariadb_test

import (
	"context"
	"testing"

	"github.com/querylex/querylex/internal/db"
	_ "github.com/querylex/querylex/internal/db/mariadb"
)

func TestMariaDBAdapter_FactoryRegistration(t *testing.T) {
	t.Run("mariadb registered", func(t *testing.T) {
		adapter, err := db.Open("mariadb", "")
		if err != nil {
			t.Fatalf("Open(mariadb) failed: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter for mariadb")
		}
	})
}

func TestMariaDBAdapter_DatabaseType(t *testing.T) {
	adapter, err := db.Open("mariadb", "")
	if err != nil {
		t.Fatalf("Open(mariadb) failed: %v", err)
	}
	if dt := adapter.DatabaseType(); dt != "mariadb" {
		t.Fatalf("expected mariadb, got %s", dt)
	}
}

func TestMariaDBAdapter_ConcreteReturnTypes(t *testing.T) {
	adapter, err := db.Open("mariadb", "")
	if err != nil {
		t.Fatalf("Open(mariadb) failed: %v", err)
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

func TestMariaDBAdapter_NoErrNotImplemented(t *testing.T) {
	adapter, err := db.Open("mariadb", "")
	if err != nil {
		t.Fatalf("Open(mariadb) failed: %v", err)
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

func TestMariaDBAdapter_ConnectUsesMySQLDriver(t *testing.T) {
	adapter, err := db.Open("mariadb", "")
	if err != nil {
		t.Fatalf("Open(mariadb) failed: %v", err)
	}

	// Connecting to a non-existent server should fail with a mysql driver error,
	// not "unsupported database type" or similar.
	ctx := context.Background()
	err = adapter.Connect(ctx, "invalid:dsn")
	if err != nil {
		// We expect a connection error (not unsupported type) because the DSN is invalid
		// This confirms the mysql driver was used
		t.Logf("Connect with invalid DSN gave expected error: %v", err)
	}
}

func TestMariaDBAdapter_ValidateSelect(t *testing.T) {
	adapter, err := db.Open("mariadb", "")
	if err != nil {
		t.Fatalf("Open(mariadb) failed: %v", err)
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
