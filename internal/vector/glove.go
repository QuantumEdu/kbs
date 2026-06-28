// Package vector provides GloVe vector loading, text tokenization, and embedding.
package vector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// GloveVectors holds word-to-vector mappings loaded from a GloVe text file.
// It is safe for concurrent use once loaded.
type GloveVectors struct {
	vectors map[string][]float32
	dims    int

	mu     sync.Mutex
	once   sync.Once
	loaded bool
	loadErr error
}

// LoadGlove parses a GloVe text file and returns the populated GloveVectors.
// The file format is one word per line, space-separated floats:
//
//	word 0.123 0.456 ... 0.789
//
// All vectors in the file must have the same dimensionality.
func LoadGlove(path string) (*GloveVectors, error) {
	g := &GloveVectors{}
	return g, g.load(path)
}

// LazyLoad registers a path for deferred loading. The file is parsed on the
// first call to Embed, Dims, or Len. Safe to call multiple times — only the
// first path is used.
func (g *GloveVectors) LazyLoad(path string) {
	g.once.Do(func() {
		_ = g.load(path) // errors surfaced on next access
	})
}

func (g *GloveVectors) load(path string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.loaded {
		return g.loadErr
	}

	file, err := os.Open(path)
	if err != nil {
		g.loaded = true
		g.loadErr = fmt.Errorf("open glove file: %w", err)
		return g.loadErr
	}
	defer file.Close()

	vectors := make(map[string][]float32)
	var dims int
	scanner := bufio.NewScanner(file)
	// Increase buffer for large GloVe files (lines can be ~10KB for 300d).
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		word := fields[0]
		vec := make([]float32, len(fields)-1)
		for i, f := range fields[1:] {
			v, err := strconv.ParseFloat(f, 32)
			if err != nil {
				g.loaded = true
				g.loadErr = fmt.Errorf("parse float in word %q: %w", word, err)
				return g.loadErr
			}
			vec[i] = float32(v)
		}
		if dims == 0 {
			dims = len(vec)
		} else if len(vec) != dims {
			g.loaded = true
			g.loadErr = fmt.Errorf("word %q has %d dimensions, expected %d", word, len(vec), dims)
			return g.loadErr
		}
		vectors[word] = vec
	}

	if err := scanner.Err(); err != nil {
		g.loaded = true
		g.loadErr = fmt.Errorf("scan glove file: %w", err)
		return g.loadErr
	}

	g.vectors = vectors
	g.dims = dims
	g.loaded = true
	return nil
}

// Vector returns the embedding vector for the given word and whether it was found.
func (g *GloveVectors) Vector(word string) ([]float32, bool) {
	g.mu.Lock()
	vec, ok := g.vectors[word]
	g.mu.Unlock()
	return vec, ok
}

// Dims returns the dimensionality of the loaded vectors (e.g. 300).
// Returns 0 if no vectors have been loaded.
func (g *GloveVectors) Dims() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.dims
}

// Len returns the number of words in the vocabulary.
func (g *GloveVectors) Len() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.vectors)
}

// Loaded returns true if vectors have been successfully loaded.
func (g *GloveVectors) Loaded() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.loaded && g.loadErr == nil
}
