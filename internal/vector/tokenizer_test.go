package vector

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", []string{}},
		{"single word", "hello", []string{"hello"}},
		{"lowercase conversion", "Hello World", []string{"hello", "world"}},
		{"whitespace splitting", "hello  world", []string{"hello", "world"}},
		{"leading trailing spaces", "  hello world  ", []string{"hello", "world"}},
		{"newlines and tabs", "hello\nworld\ttest", []string{"hello", "world", "test"}},
		{"filters stop words", "the quick brown fox", []string{"quick", "brown", "fox"}},
		{"filters 'is' 'a' 'the'", "is a the test", []string{"test"}},
		{"filters punctuation", "hello! world?", []string{"hello", "world"}},
		{"filters commas and periods", "hello, world. test", []string{"hello", "world", "test"}},
		{"filters digits", "hello123 world", []string{"world"}},
		{"filters pure numbers", "123 456 abc", []string{"abc"}},
		{"filters hyphenated words", "state-of-the-art model", []string{"model"}},
		{"filters empty tokens after cleanup", "!!! ???", []string{}},
		{"mixed stop words and valid", "the quick brown fox jumps over the lazy dog",
			[]string{"quick", "brown", "fox", "jumps", "lazy", "dog"}},
		{"all stop words", "the is a an in", []string{}},
		{"acronyms kept when all-alpha after lowercase", "JWT auth token", []string{"jwt", "auth", "token"}},
		{"unicode letters filtered (non-ASCII)", "café résumé", []string{}},
		{"only non-alpha", "1234 5678 !@#", []string{}},
		{"simple sentence", "Machine learning is fun",
			[]string{"machine", "learning", "fun"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Tokenize(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Tokenize(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTokenize_Deterministic(t *testing.T) {
	// Ensure repeated calls return the same result.
	input := "The quick brown fox jumps over the lazy dog"
	first := Tokenize(input)
	for i := 0; i < 10; i++ {
		second := Tokenize(input)
		if !reflect.DeepEqual(first, second) {
			t.Errorf("Tokenize not deterministic at iteration %d: %v vs %v", i, first, second)
		}
	}
}
