package ai

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cskiller24/querylex/internal/credentials"
)

func TestResolveAIConfig_FromEnvVar(t *testing.T) {
	t.Setenv("QUERYLEX_AI_API_KEY", "sk-test-key-12345")
	t.Setenv("QUERYLEX_AI_ENDPOINT", "https://api.openai.com/v1")
	t.Setenv("QUERYLEX_AI_MODEL", "gpt-4o")
	t.Setenv("QUERYLEX_AI_EMBEDDING_MODEL", "text-embedding-3-small")
	t.Setenv("QUERYLEX_AI_MAX_TOKENS", "128000")

	cfg, err := ResolveAIConfig("/tmp")
	if err != nil {
		t.Fatalf("ResolveAIConfig returned error: %v", err)
	}
	if cfg.APIKey != "sk-test-key-12345" {
		t.Errorf("expected APIKey sk-test-key-12345, got %s", cfg.APIKey)
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("expected Model gpt-4o, got %s", cfg.Model)
	}
	if cfg.EmbeddingModel != "text-embedding-3-small" {
		t.Errorf("expected EmbeddingModel text-embedding-3-small, got %s", cfg.EmbeddingModel)
	}
	if cfg.MaxTokens != 128000 {
		t.Errorf("expected MaxTokens 128000, got %d", cfg.MaxTokens)
	}
}

func TestResolveAIConfig_MissingReturnsError(t *testing.T) {
	home := t.TempDir()

	cfg, err := ResolveAIConfig(home)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
	if cfg != nil {
		t.Fatal("expected nil config on error")
	}
}

func TestLoadAIConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai_config.json")

	original := &AIConfigFile{
		Provider:       "openai",
		Model:          "gpt-4o",
		EmbeddingModel: "text-embedding-3-small",
		CredentialRef: &credentials.CredentialReference{
			Provider:   "keychain",
			Service:    "querylex",
			Account:    "ai-key",
			SecretKind: "ai-key",
		},
		MaxTokens: 128000,
	}

	if err := SaveAIConfig(path, original); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	loaded, err := LoadAIConfig(path)
	if err != nil {
		t.Fatalf("LoadAIConfig failed: %v", err)
	}

	if loaded.Provider != original.Provider {
		t.Errorf("Provider: expected %s, got %s", original.Provider, loaded.Provider)
	}
	if loaded.Model != original.Model {
		t.Errorf("Model: expected %s, got %s", original.Model, loaded.Model)
	}
	if loaded.CredentialRef.Account != original.CredentialRef.Account {
		t.Errorf("CredentialRef.Account: expected %s, got %s", original.CredentialRef.Account, loaded.CredentialRef.Account)
	}
}

func TestSaveAIConfig_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai_config.json")

	cfg := &AIConfigFile{
		Provider: "openai",
		Model:    "gpt-4o",
		CredentialRef: &credentials.CredentialReference{
			Provider:   "keychain",
			Service:    "querylex",
			Account:    "ai-key",
			SecretKind: "ai-key",
		},
	}

	if err := SaveAIConfig(path, cfg); err != nil {
		t.Fatalf("SaveAIConfig failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("ai_config.json was not created")
	}
}

func TestStringOr(t *testing.T) {
	if got := stringOr("hello", "default"); got != "hello" {
		t.Errorf("stringOr('hello', 'default') = %s, want 'hello'", got)
	}
	if got := stringOr("", "default"); got != "default" {
		t.Errorf("stringOr('', 'default') = %s, want 'default'", got)
	}
}

func TestIntOr(t *testing.T) {
	if got := intOr("42", 0); got != 42 {
		t.Errorf("intOr('42', 0) = %d, want 42", got)
	}
	if got := intOr("", 10); got != 10 {
		t.Errorf("intOr('', 10) = %d, want 10", got)
	}
	if got := intOr("notanumber", 5); got != 5 {
		t.Errorf("intOr('notanumber', 5) = %d, want 5", got)
	}
}
