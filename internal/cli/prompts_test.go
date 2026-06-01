package cli

import (
	"testing"
)

func TestDefaultPort(t *testing.T) {
	if port := DefaultPort("mysql"); port != 3306 {
		t.Fatalf("expected 3306 for mysql, got %d", port)
	}
	if port := DefaultPort("postgres"); port != 5432 {
		t.Fatalf("expected 5432 for postgres, got %d", port)
	}
	if port := DefaultPort("unknown"); port != 3306 {
		t.Fatalf("expected 3306 for unknown, got %d", port)
	}
}

func TestDBSetupAnswersStruct(t *testing.T) {
	answers := DBSetupAnswers{
		DBType:   "mysql",
		Name:     "test-db",
		Host:     "localhost",
		Port:     3306,
		Database: "testdb",
		Username: "root",
		Password: "secret",
		SSLMode:  "require",
	}
	if answers.DBType != "mysql" {
		t.Fatalf("expected mysql, got %s", answers.DBType)
	}
	if answers.Port != 3306 {
		t.Fatalf("expected 3306, got %d", answers.Port)
	}
}
