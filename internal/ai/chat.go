package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type OptimizationResponse struct {
	Issues            []Issue  `json:"issues"`
	RewriteStrategy   string   `json:"rewrite_strategy"`
	OptimizedSQL      string   `json:"optimized_sql"`
	ExpectedPlanChanges []string `json:"expected_plan_changes"`
	RequiresNewIndex  bool     `json:"requires_new_index"`
	Confidence        float64  `json:"confidence"`
}

type Issue struct {
	Code           string   `json:"code"`
	Severity       string   `json:"severity"`
	Evidence       string   `json:"evidence"`
	AffectedTables []string `json:"affected_tables"`
}

func ChatCompletion(ctx context.Context, client *openai.Client, model string, systemPrompt string, userPrompt string, useStructuredOutput bool) (*openai.ChatCompletionResponse, error) {
	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: float32(math.SmallestNonzeroFloat32),
	}

	if useStructuredOutput {
		schema, err := jsonschema.GenerateSchemaForType(OptimizationResponse{})
		if err != nil {
			return nil, fmt.Errorf("generate schema: %w", err)
		}

		req.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:   "optimization_response",
				Schema: schema,
				Strict: true,
			},
		}
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		if apiErr, ok := err.(*openai.APIError); ok {
			switch apiErr.HTTPStatusCode {
			case 401, 429, 500, 502, 503:
				return nil, fmt.Errorf("%w: %s", ErrAIServiceUnavailable, apiErr.Message)
			}
		}
		return nil, fmt.Errorf("chat completion: %w", err)
	}

	return &resp, nil
}

func ParseOptimizationResponse(resp *openai.ChatCompletionResponse) (*OptimizationResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := resp.Choices[0].Message.Content
	var result OptimizationResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse optimization response: %w", err)
	}

	return &result, nil
}
