package common //nolint:revive // see breakeven.go

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBreakEven(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		rec       Recommendation
		wantMonth float64
		wantKnown bool
	}{
		{"provider value wins", Recommendation{BreakEvenMonths: 9, CommitmentCost: 1, Term: "1yr"}, 9, true},
		{"no upfront is zero and known", Recommendation{CommitmentCost: 0, EstimatedSavings: 50, Term: "1yr"}, 0, true},
		// OD 100/mo, 1yr all-upfront 720, net savings 40/mo: gross 100/mo, 7.2 months.
		{"all upfront 1yr", Recommendation{CommitmentCost: 720, EstimatedSavings: 40, Term: "1yr"}, 7.2, true},
		// 3yr partial: upfront 360, net 30/mo, amortised 10/mo, gross 40/mo -> 9 months.
		{"partial upfront 3yr", Recommendation{CommitmentCost: 360, EstimatedSavings: 30, Term: "3yr"}, 9, true},
		{"term alias", Recommendation{CommitmentCost: 360, EstimatedSavings: 30, Term: "P3Y"}, 9, true},
		{"never pays back", Recommendation{CommitmentCost: 120, EstimatedSavings: -10, Term: "1yr"}, math.Inf(1), true},
		{"unknown term", Recommendation{CommitmentCost: 100, EstimatedSavings: 10, Term: "5yr"}, 0, false},
		{"empty term", Recommendation{CommitmentCost: 100, EstimatedSavings: 10}, 0, false},
		{"negative upfront", Recommendation{CommitmentCost: -1, EstimatedSavings: 10, Term: "1yr"}, 0, false},
		{"nan savings", Recommendation{CommitmentCost: 10, EstimatedSavings: math.NaN(), Term: "1yr"}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, known := BreakEven(tt.rec)
			assert.Equal(t, tt.wantKnown, known)
			assert.InDelta(t, tt.wantMonth, got, 1e-9)
		})
	}
}

func TestBreakEven_InvariantUnderScaling(t *testing.T) {
	t.Parallel()
	rec := Recommendation{CommitmentCost: 720, EstimatedSavings: 40, Term: "1yr"}
	before, _ := BreakEven(rec)
	after, _ := BreakEven(ScaleRecommendationCosts(rec, 0.4))
	assert.InDelta(t, before, after, 1e-9)
}
