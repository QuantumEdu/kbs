package vector

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// writeGloveFixture writes a small GloVe-format file for testing.
// Returns the path and a cleanup function.
func writeGloveFixture(t *testing.T, lines []string) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "glove.fixture.txt")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path, func() {}
}

// smallFixture returns the expected 5-word, 10-dim fixture content and expected map.
func smallFixture() ([]string, map[string][]float32) {
	lines := []string{
		"the 0.010 0.020 0.030 0.040 0.050 0.060 0.070 0.080 0.090 0.100",
		"hello 0.100 0.200 0.300 0.400 0.500 0.600 0.700 0.800 0.900 0.999",
		"world 0.001 0.002 0.003 0.004 0.005 0.006 0.007 0.008 0.009 0.010",
		"computer 0.500 0.400 0.300 0.200 0.100 0.000 -0.100 -0.200 -0.300 -0.400",
		"algorithm -0.100 -0.200 -0.300 -0.400 -0.500 -0.600 -0.700 -0.800 -0.900 -0.999",
	}
	expected := map[string][]float32{
		"the":       {0.010, 0.020, 0.030, 0.040, 0.050, 0.060, 0.070, 0.080, 0.090, 0.100},
		"hello":     {0.100, 0.200, 0.300, 0.400, 0.500, 0.600, 0.700, 0.800, 0.900, 0.999},
		"world":     {0.001, 0.002, 0.003, 0.004, 0.005, 0.006, 0.007, 0.008, 0.009, 0.010},
		"computer":  {0.500, 0.400, 0.300, 0.200, 0.100, 0.000, -0.100, -0.200, -0.300, -0.400},
		"algorithm": {-0.100, -0.200, -0.300, -0.400, -0.500, -0.600, -0.700, -0.800, -0.900, -0.999},
	}
	return lines, expected
}

func TestLoadGlove_ParsesWordsAndVectors(t *testing.T) {
	lines, expected := smallFixture()
	path, _ := writeGloveFixture(t, lines)

	g, err := LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove: %v", err)
	}

	if g.Dims() != 10 {
		t.Errorf("expected 10 dims, got %d", g.Dims())
	}
	if g.Len() != 5 {
		t.Errorf("expected 5 vectors, got %d", g.Len())
	}

	for word, want := range expected {
		vec, ok := g.Vector(word)
		if !ok {
			t.Errorf("word %q not found", word)
			continue
		}
		if len(vec) != len(want) {
			t.Errorf("word %q: expected %d dims, got %d", word, len(want), len(vec))
			continue
		}
		for i, v := range vec {
			if math.Abs(float64(v-want[i])) > 1e-6 {
				t.Errorf("word %q dim %d: got %.3f, want %.3f", word, i, v, want[i])
			}
		}
	}
}

func TestLoadGlove_EmptyFile(t *testing.T) {
	path, _ := writeGloveFixture(t, []string{})

	g, err := LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove empty file: %v", err)
	}

	if g.Dims() != 0 {
		t.Errorf("expected 0 dims for empty file, got %d", g.Dims())
	}
	if g.Len() != 0 {
		t.Errorf("expected 0 vectors, got %d", g.Len())
	}
}

func TestLoadGlove_FileNotFound(t *testing.T) {
	_, err := LoadGlove("/nonexistent/glove.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadGlove_MissingWord(t *testing.T) {
	g, err := LoadGlove(writeGloveFixtureHelper(t))
	if err != nil {
		t.Fatalf("LoadGlove: %v", err)
	}

	_, ok := g.Vector("nonexistent")
	if ok {
		t.Error("expected missing word to return false")
	}
}

func writeGloveFixtureHelper(t *testing.T) string {
	t.Helper()
	lines, _ := smallFixture()
	path, _ := writeGloveFixture(t, lines)
	return path
}

func TestLoadGlove_SkipsBlankLines(t *testing.T) {
	content := "hello 0.1 0.2 0.3\n\nworld 0.4 0.5 0.6\n  \n"
	path, _ := writeGloveFixture(t, []string{content})

	g, err := LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove: %v", err)
	}

	if g.Len() != 2 {
		t.Errorf("expected 2 vectors, got %d", g.Len())
	}
}

func TestLoadGlove_InconsistentDimensions(t *testing.T) {
	lines := []string{
		"hello 0.1 0.2 0.3",
		"world 0.4 0.5",
	}
	path, _ := writeGloveFixture(t, lines)

	_, err := LoadGlove(path)
	if err == nil {
		t.Fatal("expected error for inconsistent dimensions")
	}
}

func TestGloveVectors_Loaded(t *testing.T) {
	path := writeGloveFixtureHelper(t)
	g, err := LoadGlove(path)
	if err != nil {
		t.Fatalf("LoadGlove: %v", err)
	}

	if !g.Loaded() {
		t.Error("expected Loaded() = true after successful load")
	}
}
