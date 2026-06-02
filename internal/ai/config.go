package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/querylex/querylex/internal/credentials"
	"github.com/querylex/querylex/internal/state"
)

type AIConfig struct {
	Provider       string
	Endpoint       string
	Model          string
	EmbeddingModel string
	APIKey         string
	MaxTokens      int
}

type AIConfigFile struct {
	Provider       string                       `json:"provider"`
	Endpoint       string                       `json:"endpoint,omitempty"`
	Model          string                       `json:"model"`
	EmbeddingModel string                       `json:"embedding_model,omitempty"`
	CredentialRef  *credentials.CredentialReference `json:"credential_reference"`
	MaxTokens      int                          `json:"max_tokens,omitempty"`
}

var ErrAIConfigMissing = errors.New("AI_CONFIG_MISSING")
var ErrAIServiceUnavailable = errors.New("AI_SERVICE_UNAVAILABLE")

func ResolveAIConfig(home string) (*AIConfig, error) {
	if key := os.Getenv("QUERYLEX_AI_API_KEY"); key != "" {
		cfg := &AIConfig{
			APIKey:         key,
			Endpoint:       stringOr(os.Getenv("QUERYLEX_AI_ENDPOINT"), "https://api.openai.com/v1"),
			Model:          stringOr(os.Getenv("QUERYLEX_AI_MODEL"), "gpt-4o"),
			EmbeddingModel: stringOr(os.Getenv("QUERYLEX_AI_EMBEDDING_MODEL"), "text-embedding-3-small"),
			MaxTokens:      intOr(os.Getenv("QUERYLEX_AI_MAX_TOKENS"), 128000),
		}
		return cfg, nil
	}

	cfgFile, err := LoadAIConfig(filepath.Join(home, ".querylex", "ai_config.json"))
	if err != nil {
		return nil, fmt.Errorf("%w: AI provider not configured. Run 'querylex ai-config' to set up.", ErrAIConfigMissing)
	}

	credStore := selectCredentialStore()
	secret, err := credStore.Retrieve(cfgFile.CredentialRef)
	if err != nil {
		return nil, fmt.Errorf("%w: AI provider not configured. Run 'querylex ai-config' to set up.", ErrAIConfigMissing)
	}

	return &AIConfig{
		Provider:       cfgFile.Provider,
		Endpoint:       cfgFile.Endpoint,
		Model:          cfgFile.Model,
		EmbeddingModel: cfgFile.EmbeddingModel,
		APIKey:         secret,
		MaxTokens:      cfgFile.MaxTokens,
	}, nil
}

func LoadAIConfig(path string) (*AIConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ai config: %w", err)
	}

	var cfg AIConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse ai config: %w", err)
	}

	return &cfg, nil
}

func SaveAIConfig(path string, cfg *AIConfigFile) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal ai config: %w", err)
	}

	if err := state.AtomicWrite(path, data); err != nil {
		return fmt.Errorf("save ai config: %w", err)
	}

	return nil
}

func stringOr(val, def string) string {
	if val != "" {
		return val
	}
	return def
}

func intOr(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func selectCredentialStore() credentials.CredentialStore {
	stores := []credentials.CredentialStore{
		credentials.NewKeychainStore(),
		credentials.NewEncryptedFileStore(""),
		credentials.NewEnvStore(),
	}

	for _, s := range stores {
		if s.Available() {
			return s
		}
	}

	return credentials.NewEnvStore()
}
