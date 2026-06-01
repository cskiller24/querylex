package db_test

import (
	"context"
	"testing"

	"github.com/querylex/querylex/internal/db"
	_ "github.com/querylex/querylex/internal/db/mysql"
	_ "github.com/querylex/querylex/internal/db/postgresql"
)

func TestAdapterStubs(t *testing.T) {
	adapter, err := db.Open("mysql", "")
	if err != nil {
		t.Fatalf("Open(mysql) failed: %v", err)
	}

	stubMethods := []struct {
		name string
		fn   func() (any, error)
	}{
		{"Schema", func() (any, error) { return adapter.Schema(context.Background(), nil) }},
		{"Explain", func() (any, error) { return adapter.Explain(context.Background(), "", false) }},
		{"Validate", func() (any, error) { return adapter.Validate(context.Background(), "") }},
		{"Stats", func() (any, error) { return adapter.Stats(context.Background(), nil) }},
		{"Indexes", func() (any, error) { return adapter.Indexes(context.Background(), nil) }},
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
