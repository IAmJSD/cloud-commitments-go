package main

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateInt32Range(t *testing.T) {
	// Computed at run time so the test still compiles where int is 32 bits;
	// there it wraps to math.MinInt32, which is rejected as well.
	aboveMax := math.MaxInt32
	aboveMax++

	tests := []struct {
		name    string
		n       int
		wantErr bool
	}{
		{name: "zero", n: 0, wantErr: true},
		{name: "negative", n: -1, wantErr: true},
		{name: "lower bound", n: 1, wantErr: false},
		{name: "default", n: 5, wantErr: false},
		{name: "upper bound", n: math.MaxInt32, wantErr: false},
		{name: "above upper bound", n: aboveMax, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInt32Range("--max-list", tt.n)
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--max-list must be between 1 and 2147483647")
		})
	}
}
