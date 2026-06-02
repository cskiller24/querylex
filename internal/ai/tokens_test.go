package ai

import (
	"testing"
)

func TestAddTierUnderBudget(t *testing.T) {
	budget := NewTokenBudget(1000)
	content := "hello world"
	result := budget.AddTier("test", content)
	if result != content {
		t.Errorf("expected full content, got truncated: %q", result)
	}
	if len(budget.Warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(budget.Warnings))
	}
}

func TestAddTierTruncationAtCap(t *testing.T) {
	budget := NewTokenBudget(40)
	content := make([]byte, 200)
	for i := range content {
		content[i] = 'a'
	}
	result := budget.AddTier("test", string(content))
	if len(result) >= len(content) {
		t.Error("expected truncated content")
	}
	if len(budget.Warnings) == 0 {
		t.Error("expected warning for truncation")
	}
}

func TestAddTierFullyExhaustedReturnsEmpty(t *testing.T) {
	budget := NewTokenBudget(10)
	budget.Used = 10

	content := "any content"
	result := budget.AddTier("test", content)
	if result != "" {
		t.Errorf("expected empty string when budget exhausted, got %q", result)
	}
}

func TestAddTierWarningsAccumulated(t *testing.T) {
	budget := NewTokenBudget(40)

	bigContent := make([]byte, 200)
	for i := range bigContent {
		bigContent[i] = 'b'
	}

	budget.AddTier("first", string(bigContent))
	if len(budget.Warnings) == 0 {
		t.Error("expected at least one warning from first add")
	}

	firstWarningCount := len(budget.Warnings)

	budget.AddTier("second", string(bigContent))
	if len(budget.Warnings) <= firstWarningCount {
		t.Error("expected additional warning from second add")
	}
}

func TestNewTokenBudgetDefaults(t *testing.T) {
	b := NewTokenBudget(0)
	if b.MaxTokens != DefaultMaxTokens {
		t.Errorf("expected DefaultMaxTokens %d, got %d", DefaultMaxTokens, b.MaxTokens)
	}

	b2 := NewTokenBudget(50000)
	if b2.MaxTokens != 50000 {
		t.Errorf("expected 50000, got %d", b2.MaxTokens)
	}
}
