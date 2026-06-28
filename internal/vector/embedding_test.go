package vector

import (
	"math"
	"reflect"
	"testing"
)

// testGlove returns a pre-built GloveVectors with a small known vocabulary.
func testGlove() *GloveVectors {
	return &GloveVectors{
		vectors: map[string][]float32{
			"hello":    {0.1, 0.2, 0.3},
			"world":    {0.4, 0.5, 0.6},
			"computer": {0.7, 0.8, 0.9},
		},
		dims:   3,
		loaded: true,
	}
}

func TestEmbed_SimpleAverage(t *testing.T) {
	glove := testGlove()
	vec, err := Embed("hello world", glove)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	// Average of [0.1,0.2,0.3] and [0.4,0.5,0.6] = [0.25, 0.35, 0.45]
	expected := []float32{0.25, 0.35, 0.45}
	if len(vec) != len(expected) {
		t.Fatalf("expected %d dims, got %d", len(expected), len(vec))
	}
	for i, v := range vec {
		if math.Abs(float64(v-expected[i])) > 1e-6 {
			t.Errorf("dim %d: got %.6f, want %.6f", i, v, expected[i])
		}
	}
}

func TestEmbed_SingleWord(t *testing.T) {
	glove := testGlove()
	vec, err := Embed("hello", glove)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	expected := []float32{0.1, 0.2, 0.3}
	if !reflect.DeepEqual(vec, expected) {
		t.Errorf("got %v, want %v", vec, expected)
	}
}

func TestEmbed_AllOOV(t *testing.T) {
	glove := testGlove()
	vec, err := Embed("unknown missing words", glove)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if vec != nil {
		t.Errorf("expected nil vector for all-OOV input, got %v", vec)
	}
}

func TestEmbed_StopWordsFiltered(t *testing.T) {
	glove := testGlove()
	vec, err := Embed("hello in the world", glove)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	// "in" and "the" should be filtered; average of hello+world.
	expected := []float32{0.25, 0.35, 0.45}
	if len(vec) != len(expected) {
		t.Fatalf("expected %d dims, got %d", len(expected), len(vec))
	}
	for i, v := range vec {
		if math.Abs(float64(v-expected[i])) > 1e-6 {
			t.Errorf("dim %d: got %.6f, want %.6f", i, v, expected[i])
		}
	}
}

func TestEmbed_EmptyText(t *testing.T) {
	glove := testGlove()
	vec, err := Embed("", glove)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	if vec != nil {
		t.Errorf("expected nil vector for empty text, got %v", vec)
	}
}

func TestEmbed_NotLoaded(t *testing.T) {
	empty := &GloveVectors{}
	_, err := Embed("hello", empty)
	if err == nil {
		t.Fatal("expected error for not-loaded vectors")
	}
}

func TestEmbed_MixedKnownAndUnknown(t *testing.T) {
	glove := testGlove()
	vec, err := Embed("hello unknown world", glove)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	// "unknown" → OOV, only hello + world averaged.
	expected := []float32{0.25, 0.35, 0.45}
	if len(vec) != len(expected) {
		t.Fatalf("expected %d dims, got %d", len(expected), len(vec))
	}
	for i, v := range vec {
		if math.Abs(float64(v-expected[i])) > 1e-6 {
			t.Errorf("dim %d: got %.6f, want %.6f", i, v, expected[i])
		}
	}
}

func TestSerializeDeserialize_Roundtrip(t *testing.T) {
	tests := []struct {
		name string
		vec  []float32
	}{
		{"3d vector", []float32{0.1, 0.2, 0.3}},
		{"300d vector", make300D()},
		{"single element", []float32{42.0}},
		{"negative values", []float32{-1.5, 0.0, 3.75, -0.125}},
		{"empty", nil},
		{"zero length", []float32{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantNil := tt.vec == nil
			data := Serialize(tt.vec)

			// nil input → nil output.
			if wantNil && data != nil {
				t.Errorf("expected nil for nil input, got %v", data)
				return
			}
			// non-nil → non-nil bytes.
			if !wantNil && data == nil {
				t.Errorf("expected non-nil bytes, got nil")
				return
			}
			if len(tt.vec) > 0 && len(data) != len(tt.vec)*4 {
				t.Errorf("expected %d bytes, got %d", len(tt.vec)*4, len(data))
				return
			}

			restored, err := Deserialize(data)
			if err != nil {
				t.Fatalf("Deserialize: %v", err)
			}

			if wantNil {
				if restored != nil {
					t.Errorf("expected nil, got %v", restored)
				}
				return
			}

			if !reflect.DeepEqual(restored, tt.vec) {
				t.Errorf("roundtrip mismatch: got %v, want %v", restored, tt.vec)
			}
		})
	}
}

func TestDeserialize_InvalidLength(t *testing.T) {
	_, err := Deserialize([]byte{0x01, 0x02, 0x03}) // 3 bytes, not multiple of 4
	if err == nil {
		t.Fatal("expected error for invalid data length")
	}
}

func TestDeserialize_Empty(t *testing.T) {
	vec, err := Deserialize(nil)
	if err != nil {
		t.Fatalf("Deserialize nil: %v", err)
	}
	if vec != nil {
		t.Errorf("expected nil for nil input, got %v", vec)
	}

	vec, err = Deserialize([]byte{})
	if err != nil {
		t.Fatalf("Deserialize empty: %v", err)
	}
	if vec == nil {
		t.Errorf("expected empty non-nil slice for empty byte input, got nil")
	}
	if len(vec) != 0 {
		t.Errorf("expected empty vector, got %v", vec)
	}
}

func TestSerialize_300dSize(t *testing.T) {
	vec := make300D()
	data := Serialize(vec)
	if len(data) != 300*4 {
		t.Errorf("expected %d bytes, got %d", 300*4, len(data))
	}
}

// make300D creates a deterministic 300-dimensional test vector.
func make300D() []float32 {
	vec := make([]float32, 300)
	for i := range vec {
		vec[i] = float32(i+1) / 100.0
	}
	return vec
}
