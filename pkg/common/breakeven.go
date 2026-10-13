package common //nolint:revive // pkg/common is the module's core domain package (Recommendation, ProviderType, ...), imported by name (common.X) throughout this repo and by consumer-mcp; renaming would break every consumer import

import "math"

// BreakEven returns the number of months until the upfront payment of rec is
// recovered by its recurring discount, and whether that is known.
//
//   - A positive rec.BreakEvenMonths set by a provider wins.
//   - No upfront (CommitmentCost == 0): (0, true). There is nothing to recover.
//   - Otherwise upfront / gross monthly savings, where gross monthly savings is
//     EstimatedSavings plus the upfront amortised over the term. EstimatedSavings
//     is the net monthly figure (already after the amortised upfront), so using
//     it alone would overstate the break-even and push all-upfront commitments
//     past their own term.
//   - Net savings so low that gross monthly savings is not positive: (+Inf, true).
//     The commitment never pays back its upfront.
//   - Negative or non-finite money fields, or a term that is not 1yr/3yr:
//     (0, false). Unknown is not 0 and is never reported as a break-even.
//
// The result is invariant under ScaleRecommendationCosts: upfront and savings
// scale together.
func BreakEven(rec Recommendation) (months float64, known bool) {
	if rec.BreakEvenMonths > 0 && !math.IsInf(rec.BreakEvenMonths, 0) && !math.IsNaN(rec.BreakEvenMonths) {
		return rec.BreakEvenMonths, true
	}
	upfront, savings := rec.CommitmentCost, rec.EstimatedSavings
	if !isFinite(upfront) || !isFinite(savings) || upfront < 0 {
		return 0, false
	}
	if upfront == 0 {
		return 0, true
	}
	termMonths, ok := breakEvenTermMonths(rec.Term)
	if !ok {
		return 0, false
	}
	gross := savings + upfront/termMonths
	if gross <= 0 {
		return math.Inf(1), true
	}
	return upfront / gross, true
}

func breakEvenTermMonths(term string) (float64, bool) {
	switch normalizeReservationTerm(term) {
	case "1yr":
		return 12, true
	case "3yr":
		return 36, true
	default:
		return 0, false
	}
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
