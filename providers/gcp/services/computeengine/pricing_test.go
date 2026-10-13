package computeengine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/cloudbilling/v1"
	"google.golang.org/api/option"

	"github.com/LeanerCloud/cloud-commitments-go/pkg/common"
)

func newRec(machineType string, vcpus int, memGB float64) common.Recommendation {
	return common.Recommendation{ResourceType: machineType, Count: vcpus, Details: common.ComputeDetails{MemoryGB: memGB}}
}

var americas = []string{"us-central1", "us-east1", "us-west1"}

// catalogSKU builds a SKU shaped like the live Cloud Billing catalog entries
// (services/6F81-5844-456A, fetched 2026-10-13); descriptions, groups, units
// and prices are copied from real SKUs.
func catalogSKU(id, description, group, usageType, unit string, nanos int64, regions []string) *cloudbilling.Sku {
	return &cloudbilling.Sku{
		SkuId:          id,
		Description:    description,
		Category:       &cloudbilling.Category{ResourceFamily: "Compute", ResourceGroup: group, UsageType: usageType},
		ServiceRegions: regions,
		PricingInfo: []*cloudbilling.PricingInfo{{PricingExpression: &cloudbilling.PricingExpression{
			UsageUnit:   unit,
			TieredRates: []*cloudbilling.TierRate{{UnitPrice: &cloudbilling.Money{CurrencyCode: "USD", Nanos: nanos}}},
		}}},
	}
}

// n1Americas1YrSKUs: N1 on-demand core and RAM, then 1-year commitment core and RAM.
func n1Americas1YrSKUs() []*cloudbilling.Sku {
	return []*cloudbilling.Sku{
		catalogSKU("2E27-4F75-95CD", "N1 Predefined Instance Core running in Americas", "N1Standard", "OnDemand", "h", 31611000, americas),
		catalogSKU("6C71-E844-38BC", "N1 Predefined Instance Ram running in Americas", "N1Standard", "OnDemand", "GiBy.h", 4237000, americas),
		catalogSKU("6FDA-DB41-BB57", "Commitment v1: Cpu in Americas for 1 Year", "CPU", "Commit1Yr", "h", 19915000, americas),
		catalogSKU("E35D-222F-E443", "Commitment v1: Ram in Americas for 1 Year", "RAM", "Commit1Yr", "GiBy.h", 2669000, americas),
	}
}

// n2AmericasSKUs: live N2 SKUs for us-central1 (Americas), plus decoys the real
// catalog carries next to them.
func n2AmericasSKUs() []*cloudbilling.Sku {
	return []*cloudbilling.Sku{
		catalogSKU("n2-od-core", "N2 Instance Core running in Americas", "CPU", "OnDemand", "h", 31611000, americas),
		catalogSKU("n2-od-ram", "N2 Instance Ram running in Americas", "RAM", "OnDemand", "GiBy.h", 4237000, americas),
		catalogSKU("n2-c1-core", "Commitment v1: N2 Cpu in Americas for 1 Year", "CPU", "Commit1Yr", "h", 19915000, americas),
		catalogSKU("n2-c1-ram", "Commitment v1: N2 Ram in Americas for 1 Year", "RAM", "Commit1Yr", "GiBy.h", 2669000, americas),
		catalogSKU("n2-c3-core", "Commitment v1: N2 Cpu in Americas for 3 Year", "CPU", "Commit3Yr", "h", 14225000, americas),
		catalogSKU("n2-c3-ram", "Commitment v1: N2 Ram in Americas for 3 Year", "RAM", "Commit3Yr", "GiBy.h", 1907000, americas),
		// Same family name prefix, different product: must not be read as N2.
		catalogSKU("n2d-od-core", "N2D AMD Instance Core running in Americas", "CPU", "OnDemand", "h", 27502000, americas),
		catalogSKU("n2-custom-core", "N2 Custom Instance Core running in Americas", "CPU", "OnDemand", "h", 33000000, americas),
		catalogSKU("n2-sole-core", "Sole Tenancy Premium for N2 Sole Tenancy Instance Core running in Americas", "CPU", "OnDemand", "h", 1000000, americas),
		// Other region.
		catalogSKU("n2-od-core-eu", "N2 Instance Core running in Frankfurt", "CPU", "OnDemand", "h", 99000000, []string{"europe-west3"}),
	}
}

// Live catalog check, n2-standard-8 in us-central1 (8 vCPU, 32 GiB), 1 year:
//
//	on demand  8*0.031611 + 32*0.004237 = 0.388472 USD/h
//	commitment 8*0.019915 + 32*0.002669 = 0.244728 USD/h  (37.0% off)
func TestGetComputePricing_N2StandardFromLiveShapedCatalog(t *testing.T) {
	ctx := context.Background()
	client, _ := NewClient(ctx, "p", "us-central1")
	client.SetBillingService(&MockBillingService{skus: &cloudbilling.ListSkusResponse{Skus: n2AmericasSKUs()}})
	res := computeResources{vcpus: 8, memoryGiB: 32}

	one, err := client.getComputePricing(ctx, "n2-standard-8", "us-central1", 1, res)
	require.NoError(t, err)
	assert.Equal(t, "USD", one.Currency)
	assert.InDelta(t, 0.244728, one.HourlyRate, 1e-9)
	assert.InDelta(t, 0.244728*8760, one.CommitmentPrice, 1e-6)
	assert.InDelta(t, 0.388472*8760, one.OnDemandPrice, 1e-6)
	assert.InDelta(t, 37.0, one.SavingsPercentage, 0.05)

	three, err := client.getComputePricing(ctx, "n2-standard-8", "us-central1", 3, res)
	require.NoError(t, err)
	assert.InDelta(t, (8*0.014225+32*0.001907)*8760*3, three.CommitmentPrice, 1e-6)
	assert.InDelta(t, 0.388472*8760*3, three.OnDemandPrice, 1e-6)
	assert.Greater(t, three.SavingsPercentage, one.SavingsPercentage)
}

func TestFindFamilyRates_ErrorsAreLoud(t *testing.T) {
	t.Run("region without SKUs", func(t *testing.T) {
		_, err := findFamilyRates(n2AmericasSKUs(), "n2-standard-8", "asia-east1", 1)
		require.ErrorContains(t, err, "no on-demand core and RAM pricing")
	})
	t.Run("family without commitment SKUs", func(t *testing.T) {
		_, err := findFamilyRates(n2AmericasSKUs()[:2], "n2-standard-8", "us-central1", 1)
		require.ErrorContains(t, err, "no commitment pricing")
	})
	t.Run("unit mismatch", func(t *testing.T) {
		skus := n2AmericasSKUs()
		skus[0].PricingInfo[0].PricingExpression.UsageUnit = "mo"
		_, err := findFamilyRates(skus, "n2-standard-8", "us-central1", 1)
		require.ErrorContains(t, err, "expected \"h\"")
	})
	t.Run("conflicting duplicate", func(t *testing.T) {
		skus := append(n2AmericasSKUs(), catalogSKU("dup", "N2 Instance Core running in Americas", "CPU", "OnDemand", "h", 1, americas))
		_, err := findFamilyRates(skus, "n2-standard-8", "us-central1", 1)
		require.ErrorContains(t, err, "ambiguous catalog")
	})
	t.Run("usage type disagrees with description", func(t *testing.T) {
		skus := n2AmericasSKUs()
		skus[2].Category.UsageType = "Commit3Yr"
		_, err := findFamilyRates(skus, "n2-standard-8", "us-central1", 1)
		require.ErrorContains(t, err, "usage type")
	})
	t.Run("missing currency", func(t *testing.T) {
		skus := n2AmericasSKUs()
		skus[0].PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice.CurrencyCode = ""
		_, err := findFamilyRates(skus, "n2-standard-8", "us-central1", 1)
		require.ErrorContains(t, err, "no currency")
	})
}

func TestResourcesFromRecommendation_RequiresCountAndMemory(t *testing.T) {
	_, err := resourcesFromRecommendation(newRec("n2-standard-8", 0, 32))
	require.Error(t, err)
	_, err = resourcesFromRecommendation(newRec("n2-standard-8", 8, 0))
	require.Error(t, err)
}

// catalogServer serves nSKUs synthetic SKUs in pages of pageSize and records
// the page tokens it was asked for.
func catalogServer(t *testing.T, total, pageSize int, tokens *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("pageToken")
		*tokens = append(*tokens, token)
		start := 0
		if token != "" {
			start, _ = strconv.Atoi(token)
		}
		end := min(start+pageSize, total)
		body := `{"skus":[`
		for i := start; i < end; i++ {
			if i > start {
				body += ","
			}
			body += fmt.Sprintf(`{"skuId":"sku-%d"}`, i)
		}
		body += `]`
		if end < total {
			body += fmt.Sprintf(`,"nextPageToken":"%d"`, end)
		}
		body += `}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

// The live Compute Engine catalog is 33,453 SKUs over 7 pages of 5,000; reading
// only the first page left the commitment SKUs unseen.
func TestRealBillingService_ListSKUsFollowsEveryPage(t *testing.T) {
	var tokens []string
	srv := catalogServer(t, 33453, 5000, &tokens)
	defer srv.Close()

	api, err := cloudbilling.NewService(context.Background(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	require.NoError(t, err)
	svc := &realBillingService{ctx: context.Background(), service: api}

	resp, err := svc.ListSKUs(computeServiceID)
	require.NoError(t, err)
	assert.Len(t, resp.Skus, 33453)
	assert.Len(t, tokens, 7)
}

func TestRealBillingService_ListSKUsPageCap(t *testing.T) {
	var tokens []string
	srv := catalogServer(t, (maxSKUPages+1)*10, 10, &tokens)
	defer srv.Close()

	api, err := cloudbilling.NewService(context.Background(), option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	require.NoError(t, err)
	svc := &realBillingService{ctx: context.Background(), service: api}

	_, err = svc.ListSKUs(computeServiceID)
	require.ErrorContains(t, err, "page cap")
}
