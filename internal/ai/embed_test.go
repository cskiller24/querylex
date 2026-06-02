package ai

import (
	"math"
	"testing"
)

func TestCosineSimilarityIdentical(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{1.0, 2.0, 3.0}

	sim := CosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 0.0001 {
		t.Errorf("expected 1.0 for identical vectors, got %f", sim)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1.0, 0.0}
	b := []float32{0.0, 1.0}

	sim := CosineSimilarity(a, b)
	if math.Abs(sim-0.0) > 0.0001 {
		t.Errorf("expected 0.0 for orthogonal vectors, got %f", sim)
	}
}

func TestCosineSimilarityMismatchedLengths(t *testing.T) {
	a := []float32{1.0, 2.0}
	b := []float32{1.0}

	sim := CosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("expected 0.0 for mismatched lengths, got %f", sim)
	}
}

func TestCosineSimilarityZeroVectors(t *testing.T) {
	a := []float32{0.0, 0.0}
	b := []float32{0.0, 0.0}

	sim := CosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("expected 0.0 for zero vectors, got %f", sim)
	}
}

func TestCosineSimilarityEmptyVectors(t *testing.T) {
	a := []float32{}
	b := []float32{}

	sim := CosineSimilarity(a, b)
	if sim != 0.0 {
		t.Errorf("expected 0.0 for empty vectors, got %f", sim)
	}
}

func TestCosineSimilarityHalfSimilar(t *testing.T) {
	a := []float32{1.0, 0.0, 1.0, 0.0}
	b := []float32{1.0, 0.0, 0.0, 1.0}

	sim := CosineSimilarity(a, b)
	expected := 0.5
	if math.Abs(sim-expected) > 0.0001 {
		t.Errorf("expected %f for half-overlap, got %f", expected, sim)
	}
}
