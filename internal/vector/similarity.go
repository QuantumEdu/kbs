package vector

import (
	"math"
	"sort"
)

// Cosine computes the cosine similarity between two vectors a and b.
// Both must have the same non-zero length. Returns a value in [-1, 1].
// Returns 0 for mismatched lengths, empty vectors, or zero-magnitude vectors.
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		magA += ai * ai
		magB += bi * bi
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

// ScoredEntry pairs an entry ID with its cosine similarity score.
type ScoredEntry struct {
	EntryID string
	Score   float64
}

// Search performs brute-force cosine similarity search. It returns the top-k
// candidates ranked by similarity score descending. Empty candidate vectors
// are silently skipped. If limit is 0 or negative, all results are returned.
func Search(query []float32, candidates map[string][]float32, limit int) []ScoredEntry {
	results := make([]ScoredEntry, 0, len(candidates))
	for id, vec := range candidates {
		if len(vec) == 0 {
			continue
		}
		score := Cosine(query, vec)
		results = append(results, ScoredEntry{EntryID: id, Score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}
