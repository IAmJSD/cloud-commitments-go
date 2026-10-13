package common //nolint:revive // see resize.go

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResizeCount(t *testing.T) {
	t.Parallel()
	monthly := 30.0
	rec := Recommendation{
		Count: 3, CommitmentCost: 300, OnDemandCost: 600, EstimatedSavings: 150,
		SavingsPercentage: 25, BreakEvenMonths: 4, RecommendedCount: 3, RecurringMonthlyCost: &monthly,
	}

	got, err := ResizeCount(rec, 1)

	require.NoError(t, err)
	assert.Equal(t, 1, got.Count)
	assert.InDelta(t, 100.0, got.CommitmentCost, 1e-9)
	assert.InDelta(t, 200.0, got.OnDemandCost, 1e-9)
	assert.InDelta(t, 50.0, got.EstimatedSavings, 1e-9)
	require.NotNil(t, got.RecurringMonthlyCost)
	assert.InDelta(t, 10.0, *got.RecurringMonthlyCost, 1e-9)
	assert.Equal(t, 25.0, got.SavingsPercentage)
	assert.Equal(t, 4.0, got.BreakEvenMonths)
	assert.Equal(t, 3, got.RecommendedCount, "frozen pre-sizing record")
	assert.Equal(t, 30.0, monthly, "input pointer untouched")
}

func TestResizeCount_NilRecurringStaysNil(t *testing.T) {
	t.Parallel()
	got, err := ResizeCount(Recommendation{Count: 2, CommitmentCost: 10}, 1)
	require.NoError(t, err)
	assert.Nil(t, got.RecurringMonthlyCost)
}

func TestResizeCount_SameCountIsIdentity(t *testing.T) {
	t.Parallel()
	rec := Recommendation{Count: 2, CommitmentCost: 10}
	got, err := ResizeCount(rec, 2)
	require.NoError(t, err)
	assert.Equal(t, rec, got)
}

func TestResizeCount_RejectsInvalid(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		rec Recommendation
		n   int
	}{
		"zero source count": {Recommendation{Count: 0, CommitmentCost: 10}, 1},
		"zero target":       {Recommendation{Count: 2}, 0},
		"negative target":   {Recommendation{Count: 2}, -1},
		"growing":           {Recommendation{Count: 2}, 3},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := ResizeCount(tc.rec, tc.n)
			assert.Error(t, err)
		})
	}
}
