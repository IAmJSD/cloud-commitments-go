// Package skulist reads every page of a Cloud Billing catalog SKU listing.
package skulist

import (
	"context"
	"fmt"

	"google.golang.org/api/cloudbilling/v1"
)

// DefaultMaxPages bounds one listing. The largest catalog service seen live
// (Compute Engine) spans 7 pages of 5000 SKUs.
const DefaultMaxPages = 200

// All returns the SKUs of every page of call. It errors, never returning a
// partial list, if the listing needs more than maxPages pages.
func All(ctx context.Context, call *cloudbilling.ServicesSkusListCall, maxPages int) ([]*cloudbilling.Sku, error) {
	var skus []*cloudbilling.Sku
	pages := 0
	err := call.Pages(ctx, func(resp *cloudbilling.ListSkusResponse) error {
		pages++
		if pages > maxPages {
			return fmt.Errorf("SKU listing exceeded %d pages", maxPages)
		}
		skus = append(skus, resp.Skus...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return skus, nil
}
