package agenttelemetry

import (
	"strings"
	"unicode"
)

// EntropyScanner detects high-entropy tokens in text (e.g., base64-encoded
// secrets). A token is flagged if the ratio of alphanumeric characters to
// total characters exceeds minRatio AND the token length exceeds minLength.
type EntropyScanner struct {
	minLength int     // minimum token length to consider (default: 20)
	minRatio  float64 // minimum alphanumeric ratio to flag (default: 0.75)
}

// NewEntropyScanner creates a scanner with default thresholds: minLength=20,
// minRatio=0.75.
func NewEntropyScanner() *EntropyScanner {
	return &EntropyScanner{
		minLength: 20,
		minRatio:  0.75,
	}
}

// delimiters is the set of characters trimmed from token edges.
const delimiters = `"',;:[]{}()`

// Scan returns true if any token in text exceeds the entropy ratio threshold.
//
// Tokenization: split by whitespace, then trim delimiter characters from
// both ends of each token. Tokens shorter than minLength are ignored.
//
// Ratio: count of alphanumeric characters (a-z, A-Z, 0-9) divided by total
// characters in the token.
func (s *EntropyScanner) Scan(text string) bool {
	for _, raw := range strings.Fields(text) {
		token := strings.Trim(raw, delimiters)
		if len(token) < s.minLength {
			continue
		}
		alpha := 0
		for _, r := range token {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				alpha++
			}
		}
		if float64(alpha)/float64(len(token)) > s.minRatio {
			return true
		}
	}
	return false
}
