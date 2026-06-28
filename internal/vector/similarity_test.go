package vector

import (
	"math"
	"testing"
)

func TestCosine_Identical(t *testing.T) {
	v := []float32{1.0, 2.0, 3.0}
	score := Cosine(v, v)
	if math.Abs(score-1.0) > 1e-6 {
		t.Errorf("expected 1.0 for identical vectors, got %f", score)
	}
}

func TestCosine_Orthogonal(t *testing.T) {
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{0.0, 1.0, 0.0}
	score := Cosine(a, b)
	if math.Abs(score) > 1e-6 {
		t.Errorf("expected ~0 for orthogonal vectors, got %f", score)
	}
}

func TestCosine_Opposite(t *testing.T) {
	a := []float32{1.0, 0.0}
	b := []float32{-1.0, 0.0}
	score := Cosine(a, b)
	if math.Abs(score-(-1.0)) > 1e-6 {
		t.Errorf("expected -1.0 for opposite vectors, got %f", score)
	}
}

func TestCosine_DifferentLengths(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{1.0, 2.0}
	score := Cosine(a, b)
	if score != 0 {
		t.Errorf("expected 0 for mismatched lengths, got %f", score)
	}
}

func TestCosine_Empty(t *testing.T) {
	score := Cosine(nil, nil)
	if score != 0 {
		t.Errorf("expected 0 for empty vectors, got %f", score)
	}
}

func TestCosine_ZeroMagnitude(t *testing.T) {
	a := []float32{0.0, 0.0, 0.0}
	b := []float32{1.0, 2.0, 3.0}
	score := Cosine(a, b)
	if score != 0 {
		t.Errorf("expected 0 when one vector has zero magnitude, got %f", score)
	}
}

func TestSearch_Ranking(t *testing.T) {
	query := []float32{1.0, 0.0, 0.0}
	candidates := map[string][]float32{
		"a": {1.0, 0.0, 0.0},   // cos=1.0
		"b": {0.0, 1.0, 0.0},   // cos=0.0
		"c": {0.5, 0.5, 0.0},   // cos≈0.707
	}
	results := Search(query, candidates, 0)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].EntryID != "a" {
		t.Errorf("expected 'a' first, got %s", results[0].EntryID)
	}
	if results[1].EntryID != "c" {
		t.Errorf("expected 'c' second, got %s", results[1].EntryID)
	}
	if results[2].EntryID != "b" {
		t.Errorf("expected 'b' third, got %s", results[2].EntryID)
	}
}

func TestSearch_Limited(t *testing.T) {
	query := []float32{1.0, 0.0}
	candidates := map[string][]float32{
		"x": {1.0, 0.0},
		"y": {0.9, 0.1},
		"z": {0.5, 0.5},
	}
	results := Search(query, candidates, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestSearch_EmptyCandidates(t *testing.T) {
	results := Search([]float32{1.0, 0.0}, map[string][]float32{}, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty candidates, got %d", len(results))
	}
}

func TestSearch_EmptyEmbeddingsFiltered(t *testing.T) {
	query := []float32{1.0, 0.0}
	candidates := map[string][]float32{
		"a": {}, // empty vector — should be skipped
		"b": {1.0, 0.0},
	}
	results := Search(query, candidates, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (empty filtered), got %d", len(results))
	}
	if results[0].EntryID != "b" {
		t.Errorf("expected 'b', got %s", results[0].EntryID)
	}
}
