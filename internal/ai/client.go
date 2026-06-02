package ai

import (
	openai "github.com/sashabaranov/go-openai"
)

func NewClient(config *AIConfig) *openai.Client {
	cfg := openai.DefaultConfig(config.APIKey)
	if config.Endpoint != "" {
		cfg.BaseURL = config.Endpoint
	}
	return openai.NewClientWithConfig(cfg)
}
