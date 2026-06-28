package vector

import (
	"strings"
	"unicode"
)

// stopWords is a set of common English stop words filtered during tokenization.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true,
	"and": true, "or": true, "but": true,
	"is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true,
	"in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true,
	"from": true, "it": true, "its": true,
	"that": true, "this": true, "these": true, "those": true,
	"i": true, "you": true, "he": true, "she": true,
	"we": true, "they": true, "me": true, "him": true,
	"her": true, "us": true, "them": true,
	"over": true, "as": true, "if": true, "so": true,
	"no": true, "not": true, "can": true, "will": true,
	"just": true, "do": true, "does": true, "did": true,
	"has": true, "have": true, "had": true, "my": true, "your": true,
}

// Tokenize converts text to a list of lowercase tokens, filtering out
// stop words and tokens containing non-alpha characters. Empty tokens
// and tokens consisting solely of whitespace/punctuation are discarded.
func Tokenize(text string) []string {
	// Lowercase and split on whitespace.
	lower := strings.ToLower(text)
	raw := strings.Fields(lower)

	tokens := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.TrimFunc(t, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if t == "" {
			continue
		}
		// Filter tokens with non-alpha characters (allow only letters).
		if !isAlpha(t) {
			continue
		}
		// Filter stop words.
		if stopWords[t] {
			continue
		}
		tokens = append(tokens, t)
	}
	if len(tokens) == 0 {
		return []string{}
	}
	return tokens
}

// isAlpha returns true if every rune in s is an ASCII letter.
func isAlpha(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
