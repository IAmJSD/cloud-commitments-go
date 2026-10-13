package cloudsql

import (
	"context"
	"testing"

	"cloud.google.com/go/recommender/apiv1/recommenderpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/cloudbilling/v1"

	"github.com/LeanerCloud/cloud-commitments-go/pkg/common"
)

func activeRecs(n int) []*recommenderpb.Recommendation {
	recs := make([]*recommenderpb.Recommendation, n)
	for i := range recs {
		recs[i] = &recommenderpb.Recommendation{
			StateInfo: &recommenderpb.RecommendationStateInfo{State: recommenderpb.RecommendationStateInfo_ACTIVE},
		}
	}
	return recs
}

// A listing that spans several pages but stays under the page cap must be
// returned in full. Before the fix the cap applied per item (20), so any
// region with more than 20 recommendations failed outright.
func TestGetRecommendations_ManyItemsWithinPageCap(t *testing.T) {
	n := 3 * recsPageSize
	c, _ := NewClient(context.Background(), "p", "us-central1")
	c.SetRecommenderClient(&MockRecommenderClient{iterator: &MockRecommenderIterator{recommendations: activeRecs(n)}})
	c.SetBillingService(&MockBillingService{skus: &cloudbilling.ListSkusResponse{}})

	recs, err := c.GetRecommendations(context.Background(), &common.RecommendationParams{})
	require.NoError(t, err)
	assert.Len(t, recs, n)
}

// Exactly maxRecsItems recommendations are within the cap; the old per-item
// check rejected anything past 20.
func TestGetRecommendations_ExactlyAtCapSucceeds(t *testing.T) {
	c, _ := NewClient(context.Background(), "p", "us-central1")
	c.SetRecommenderClient(&MockRecommenderClient{iterator: &MockRecommenderIterator{recommendations: activeRecs(maxRecsItems)}})
	c.SetBillingService(&MockBillingService{skus: &cloudbilling.ListSkusResponse{}})

	recs, err := c.GetRecommendations(context.Background(), &common.RecommendationParams{})
	require.NoError(t, err)
	assert.Len(t, recs, maxRecsItems)
}

// A genuine cap breach still fails loud and returns no partial list.
func TestGetRecommendations_PastCapFails(t *testing.T) {
	c, _ := NewClient(context.Background(), "p", "us-central1")
	c.SetRecommenderClient(&MockRecommenderClient{iterator: &MockRecommenderIterator{recommendations: activeRecs(maxRecsItems + 1)}})
	c.SetBillingService(&MockBillingService{skus: &cloudbilling.ListSkusResponse{}})

	recs, err := c.GetRecommendations(context.Background(), &common.RecommendationParams{})
	require.Error(t, err)
	assert.Nil(t, recs)
}

// The cap is expressed in pages, so the request must ask for the page size
// the cap assumes.
type reqCapture struct {
	RecommenderClient
	req *recommenderpb.ListRecommendationsRequest
}

func (r *reqCapture) ListRecommendations(ctx context.Context, req *recommenderpb.ListRecommendationsRequest) RecommenderIterator {
	r.req = req
	return r.RecommenderClient.ListRecommendations(ctx, req)
}

func TestGetRecommendations_RequestsExplicitPageSize(t *testing.T) {
	c, _ := NewClient(context.Background(), "p", "us-central1")
	capture := &reqCapture{RecommenderClient: &MockRecommenderClient{iterator: &MockRecommenderIterator{recommendations: nil}}}
	c.SetRecommenderClient(capture)

	_, err := c.GetRecommendations(context.Background(), &common.RecommendationParams{})
	require.NoError(t, err)
	require.NotNil(t, capture.req)
	assert.EqualValues(t, recsPageSize, capture.req.PageSize)
}

// Pricing N recommendations lists the SKU catalog once.
type countingBilling struct {
	BillingService
	calls int
}

func (b *countingBilling) ListSKUs(id string) (*cloudbilling.ListSkusResponse, error) {
	b.calls++
	return b.BillingService.ListSKUs(id)
}

func TestListSKUs_CachedPerClient(t *testing.T) {
	b := &countingBilling{BillingService: &MockBillingService{skus: &cloudbilling.ListSkusResponse{}}}
	c, _ := NewClient(context.Background(), "p", "us-central1")
	for i := 0; i < 3; i++ {
		_, err := c.listSKUs(b)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, b.calls)
}
