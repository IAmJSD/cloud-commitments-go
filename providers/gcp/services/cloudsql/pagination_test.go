package cloudsql

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/recommender/apiv1/recommenderpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/cloudbilling/v1"
	"google.golang.org/api/option"

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

// A genuine cap breach still fails loud.
func TestGetRecommendations_PageCapBreachFails(t *testing.T) {
	c, _ := NewClient(context.Background(), "p", "us-central1")
	c.SetRecommenderClient(&MockRecommenderClient{iterator: &MockRecommenderIterator{recommendations: activeRecs(maxRecsItems + 1)}})
	c.SetBillingService(&MockBillingService{skus: &cloudbilling.ListSkusResponse{}})

	_, err := c.GetRecommendations(context.Background(), &common.RecommendationParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cap reached")
}

func TestRealBillingService_ListSKUs_FollowsNextPageToken(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := r.URL.Query().Get("pageToken")
		tokens = append(tokens, tok)
		w.Header().Set("Content-Type", "application/json")
		switch tok {
		case "":
			fmt.Fprint(w, `{"skus":[{"skuId":"A"}],"nextPageToken":"t2"}`)
		case "t2":
			fmt.Fprint(w, `{"skus":[{"skuId":"B"}],"nextPageToken":"t3"}`)
		default:
			fmt.Fprint(w, `{"skus":[{"skuId":"C"}]}`)
		}
	}))
	defer srv.Close()

	api, err := cloudbilling.NewService(context.Background(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	require.NoError(t, err)
	resp, err := (&realBillingService{service: api}).ListSKUs("services/x")
	require.NoError(t, err)
	require.Len(t, resp.Skus, 3)
	assert.Equal(t, []string{"", "t2", "t3"}, tokens)
}
