package core

import (
	"math"
	"testing"
)

func TestPowerHeuristic(t *testing.T) {
	tests := []struct {
		name     string
		nf       int
		fPdf     float64
		ng       int
		gPdf     float64
		expected float64
	}{
		{
			name:     "Equal PDFs",
			nf:       1,
			fPdf:     0.5,
			ng:       1,
			gPdf:     0.5,
			expected: 0.5,
		},
		{
			name:     "First PDF zero",
			nf:       1,
			fPdf:     0.0,
			ng:       1,
			gPdf:     0.5,
			expected: 0.0,
		},
		{
			name:     "Second PDF zero",
			nf:       1,
			fPdf:     0.5,
			ng:       1,
			gPdf:     0.0,
			expected: 1.0,
		},
		{
			name:     "First PDF higher",
			nf:       1,
			fPdf:     0.8,
			ng:       1,
			gPdf:     0.2,
			expected: 0.941176, // (0.8²) / (0.8² + 0.2²)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PowerHeuristic(tt.nf, tt.fPdf, tt.ng, tt.gPdf)
			if math.Abs(result-tt.expected) > 1e-5 {
				t.Errorf("PowerHeuristic: got %f, expected %f", result, tt.expected)
			}
		})
	}
}

func TestBalanceHeuristic(t *testing.T) {
	tests := []struct {
		name     string
		nf       int
		fPdf     float64
		ng       int
		gPdf     float64
		expected float64
	}{
		{
			name:     "Equal PDFs",
			nf:       1,
			fPdf:     0.5,
			ng:       1,
			gPdf:     0.5,
			expected: 0.5,
		},
		{
			name:     "First PDF zero",
			nf:       1,
			fPdf:     0.0,
			ng:       1,
			gPdf:     0.5,
			expected: 0.0,
		},
		{
			name:     "Second PDF zero",
			nf:       1,
			fPdf:     0.5,
			ng:       1,
			gPdf:     0.0,
			expected: 1.0,
		},
		{
			name:     "First PDF higher",
			nf:       1,
			fPdf:     0.8,
			ng:       1,
			gPdf:     0.2,
			expected: 0.8, // 0.8 / (0.8 + 0.2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BalanceHeuristic(tt.nf, tt.fPdf, tt.ng, tt.gPdf)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("BalanceHeuristic: got %f, expected %f", result, tt.expected)
			}
		})
	}
}
