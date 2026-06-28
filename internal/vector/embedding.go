package vector

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Embed computes a document-level embedding by averaging the GloVe word vectors
// of the tokens in text. Returns a zero-length vector if no token has a known
// vector or if the text produces no valid tokens.
func Embed(text string, glove *GloveVectors) ([]float32, error) {
	if !glove.Loaded() {
		return nil, fmt.Errorf("glove vectors not loaded")
	}

	tokens := Tokenize(text)
	if len(tokens) == 0 {
		return nil, nil
	}

	dims := glove.Dims()
	if dims == 0 {
		return nil, nil
	}

	sum := make([]float64, dims)
	count := 0

	for _, token := range tokens {
		vec, ok := glove.Vector(token)
		if !ok {
			continue
		}
		for i, v := range vec {
			sum[i] += float64(v)
		}
		count++
	}

	if count == 0 {
		return nil, nil
	}

	// Average and convert back to float32.
	result := make([]float32, dims)
	for i := range sum {
		result[i] = float32(sum[i] / float64(count))
	}

	return result, nil
}

// Serialize encodes a []float32 vector as a byte slice for BLOB storage.
// Uses little-endian encoding: 4 bytes per float32 value.
// Returns nil only for nil input; empty []float32{} produces []byte{}.
func Serialize(vec []float32) []byte {
	if vec == nil {
		return nil
	}
	if len(vec) == 0 {
		return []byte{}
	}

	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// Deserialize decodes a byte slice (produced by Serialize) back into a
// []float32 vector. Returns nil only for nil input.
func Deserialize(data []byte) ([]float32, error) {
	if data == nil {
		return nil, nil
	}
	if len(data) == 0 {
		return []float32{}, nil
	}
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("invalid embedding data: length %d is not a multiple of 4", len(data))
	}
	n := len(data) / 4
	vec := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4:])
		vec[i] = math.Float32frombits(bits)
	}
	return vec, nil
}
