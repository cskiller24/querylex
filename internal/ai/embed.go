package ai

import (
	"context"
	"fmt"
	"math"

	openai "github.com/sashabaranov/go-openai"
)

func GenerateEmbedding(ctx context.Context, client *openai.Client, model string, text string) ([]float32, error) {
	resp, err := client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(model),
	})
	if err != nil {
		return nil, fmt.Errorf("create embedding: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	return resp.Data[0].Embedding, nil
}

func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		fa := float64(a[i])
		fb := float64(b[i])
		dotProduct += fa * fb
		normA += fa * fa
		normB += fb * fb
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
