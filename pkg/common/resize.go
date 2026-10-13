package common //nolint:revive // pkg/common is the module's core domain package (Recommendation, ProviderType, ...), imported by name (common.X) throughout this repo and by consumer-mcp; renaming would break every consumer import

import "fmt"

// ResizeCount returns rec sized to newCount with every money field scaled by
// newCount/rec.Count (see ScaleRecommendationCosts for what scales). Use it
// wherever a filter shrinks a recommendation's count, so Count and cost
// cannot drift apart.
//
// It fails loud rather than inventing a per-unit cost: rec.Count must be
// positive, and newCount must be in [1, rec.Count]. Growing a count is the
// sizing paths' job (they derive the target from demand) and dropping to zero
// is the caller's drop decision, so neither is accepted here.
func ResizeCount(rec Recommendation, newCount int) (Recommendation, error) {
	if rec.Count <= 0 {
		return rec, fmt.Errorf("cannot resize %s/%s/%s: source count %d has no per-unit cost", rec.Service, rec.Region, rec.ResourceType, rec.Count)
	}
	if newCount < 1 || newCount > rec.Count {
		return rec, fmt.Errorf("cannot resize %s/%s/%s from %d to %d: new count must be in [1, %d]", rec.Service, rec.Region, rec.ResourceType, rec.Count, newCount, rec.Count)
	}
	if newCount == rec.Count {
		return rec, nil
	}
	resized := ScaleRecommendationCosts(rec, float64(newCount)/float64(rec.Count))
	resized.Count = newCount
	return resized, nil
}
