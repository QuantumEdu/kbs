package agenttelemetry

// TokenCounter estimates tokens and computes efficiency ratios.
type TokenCounter struct{}

// NewTokenCounter creates a new TokenCounter.
func NewTokenCounter() *TokenCounter {
	return &TokenCounter{}
}

// Estimate estimates tokens from a string using the char-div-4 rule.
// Returns the estimated token count, minimum 1 for non-empty input.
func (tc *TokenCounter) Estimate(text string) int {
	if text == "" {
		return 0
	}
	n := len(text) / 4
	if n < 1 {
		n = 1
	}
	return n
}

// EfficiencyRatio calculates the efficiency ratio as output_tokens / input_tokens.
// A low ratio (< 0.05) indicates very little output relative to input.
// If inputTokens is 0, ratio is 0 and isLow is true.
func (tc *TokenCounter) EfficiencyRatio(inputTokens, outputTokens int64) (ratio float64, isLow bool) {
	if inputTokens <= 0 {
		return 0, true
	}
	ratio = float64(outputTokens) / float64(inputTokens)
	return ratio, ratio < 0.05
}
