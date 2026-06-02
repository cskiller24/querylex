package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cskiller24/querylex/internal/credentials"
	"github.com/cskiller24/querylex/internal/state"
	"golang.org/x/term"
)

type AISetupAnswers struct {
	Provider       string
	Endpoint       string
	Model          string
	EmbeddingModel string
	APIKey         string
	MaxTokens      string
}

type aiConfigMeta struct {
	Provider       string                       `json:"provider"`
	Endpoint       string                       `json:"endpoint,omitempty"`
	Model          string                       `json:"model"`
	EmbeddingModel string                       `json:"embedding_model,omitempty"`
	CredentialRef  *credentials.CredentialReference `json:"credential_reference"`
	MaxTokens      int                          `json:"max_tokens"`
}

func PromptAISetup() (*AISetupAnswers, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("querylex ai-config requires an interactive terminal")
	}

	answers := &AISetupAnswers{}

	providerQs := &survey.Select{
		Message: "Which AI provider?",
		Options: []string{"OpenAI", "Other (OpenAI-compatible)"},
		Default: "OpenAI",
	}
	if err := survey.AskOne(providerQs, &answers.Provider); err != nil {
		return nil, err
	}

	var questions []*survey.Question
	if answers.Provider == "Other (OpenAI-compatible)" {
		questions = append(questions, &survey.Question{
			Name: "endpoint",
			Prompt: &survey.Input{
				Message: "Endpoint URL:",
				Default: "http://localhost:11434/v1",
			},
			Validate: survey.Required,
		})
	}

	questions = append(questions,
		&survey.Question{
			Name: "model",
			Prompt: &survey.Input{
				Message: "Model:",
				Default: "gpt-4o",
			},
			Validate: survey.Required,
		},
		&survey.Question{
			Name: "embedding_model",
			Prompt: &survey.Input{
				Message: "Embedding model (press Enter for default):",
				Default: "",
			},
		},
	)

	raw := struct {
		Endpoint       string
		Model          string
		EmbeddingModel string
	}{}

	if err := survey.Ask(questions, &raw); err != nil {
		return nil, err
	}

	answers.Endpoint = raw.Endpoint
	answers.Model = raw.Model
	answers.EmbeddingModel = raw.EmbeddingModel

	apiKeyQs := &survey.Password{
		Message: "API key:",
	}
	if err := survey.AskOne(apiKeyQs, &answers.APIKey, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	maxTokensQs := &survey.Input{
		Message: "Max context tokens:",
		Default: "128000",
	}
	if err := survey.AskOne(maxTokensQs, &answers.MaxTokens, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	return answers, nil
}

func RunAIConfig() error {
	answers, err := PromptAISetup()
	if err != nil {
		return fmt.Errorf("setup cancelled: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	credStore, err := credentials.SelectCredentialStore()
	if err != nil {
		return fmt.Errorf("No credential store available. Set QUERYLEX_AI_API_KEY environment variable instead.")
	}

	// If the encrypted file store was selected, prompt for passphrase
	if encStore, ok := credStore.(*credentials.EncryptedFileStore); ok {
		if err := promptEncryptedFilePassphrase(encStore, "ai"); err != nil {
			return fmt.Errorf("cannot unlock credential store: %w", err)
		}
	}

	providerName := strings.ToLower(strings.ReplaceAll(answers.Provider, " ", "-"))
	if strings.HasPrefix(providerName, "other") {
		providerName = "openai-compatible"
	}

	account := fmt.Sprintf("ai/%s", providerName)
	credRef, err := credStore.Store(account, answers.APIKey)
	if err != nil {
		return fmt.Errorf("failed to store credential: %w", err)
	}
	credRef.SecretKind = "ai-key"

	maxTokens, err := strconv.Atoi(answers.MaxTokens)
	if err != nil || maxTokens <= 0 {
		maxTokens = 128000
	}

	if answers.EmbeddingModel == "" {
		answers.EmbeddingModel = "text-embedding-3-small"
	}

	meta := aiConfigMeta{
		Provider:       providerName,
		Model:          answers.Model,
		EmbeddingModel: answers.EmbeddingModel,
		CredentialRef:  credRef,
		MaxTokens:      maxTokens,
	}

	metaData, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	aiConfigPath := filepath.Join(home, ".querylex", "ai_config.json")
	if err := state.AtomicWrite(aiConfigPath, metaData); err != nil {
		return fmt.Errorf("failed to write ai config: %w", err)
	}

	fmt.Println("AI configuration saved successfully.")
	return nil
}
