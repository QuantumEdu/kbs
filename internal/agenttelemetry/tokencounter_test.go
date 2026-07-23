package agenttelemetry

import (
	"math"
	"testing"
)

func TestTokenCounterEstimate(t *testing.T) {
	tc := NewTokenCounter()

	tests := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"short", "hi", 1},       // 2/4=0 → min 1
		{"four chars", "abcd", 1}, // 4/4=1
		{"five chars", "abcde", 1}, // 5/4=1
		{"eight chars", "abcdefgh", 2}, // 8/4=2
		{"nine chars", "abcdefghi", 2}, // 9/4=2
		{"twelve chars", "abcdefghijkl", 3}, // 12/4=3
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tc.Estimate(tt.text)
			if got != tt.want {
				t.Errorf("Estimate(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestTokenCounterEfficiencyRatio(t *testing.T) {
	tc := NewTokenCounter()

	tests := []struct {
		name    string
		input   int64
		output  int64
		wantLow bool
	}{
		{"high efficiency", 1000, 500, false},     // 0.5 → not low
		{"very low efficiency", 1000, 40, true},   // 0.04 → low
		{"boundary low", 1000, 50, false},         // 0.05 → not low
		{"boundary just below", 1000, 49, true},   // 0.049 → low
		{"zero input", 0, 100, true},              // input 0 → low
		{"negative input", -1, 100, true},         // negative input → low
		{"zero output", 1000, 0, true},            // 0 / 1000 = 0 → low
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, isLow := tc.EfficiencyRatio(tt.input, tt.output)
			if isLow != tt.wantLow {
				t.Errorf("EfficiencyRatio(%d, %d) isLow = %v, want %v", tt.input, tt.output, isLow, tt.wantLow)
			}
			if tt.input > 0 {
				expectedRatio := float64(tt.output) / float64(tt.input)
				if math.Abs(ratio-expectedRatio) > 0.0001 {
					t.Errorf("EfficiencyRatio(%d, %d) ratio = %f, want %f", tt.input, tt.output, ratio, expectedRatio)
				}
			}
		})
	}
}
