package ai

import (
	"fmt"
	"math"
)

const TokenCapPercent = 0.80
const CharPerToken = 4.0
const DefaultMaxTokens = 128000

type TokenBudget struct {
	MaxTokens int
	Used      int
	Warnings  []string
}

func NewTokenBudget(maxTokens int) *TokenBudget {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	return &TokenBudget{MaxTokens: maxTokens}
}

func (b *TokenBudget) AddTier(name, content string) string {
	estimated := int(math.Ceil(float64(len(content)) / CharPerToken))
	cap := int(float64(b.MaxTokens) * TokenCapPercent)

	if b.Used+estimated > cap {
		warning := fmt.Sprintf("%s truncated: %d tokens exceeds budget (cap: %d)", name, b.Used+estimated, cap)
		b.Warnings = append(b.Warnings, warning)

		remaining := cap - b.Used
		if remaining <= 0 {
			b.Used = cap
			return ""
		}

		truncLen := int(float64(remaining) * CharPerToken)
		if truncLen <= 0 {
			return ""
		}
		if truncLen >= len(content) {
			b.Used += estimated
			return content
		}
		truncated := content[:truncLen]
		b.Used += int(math.Ceil(float64(len(truncated)) / CharPerToken))
		return truncated
	}

	b.Used += estimated
	return content
}
