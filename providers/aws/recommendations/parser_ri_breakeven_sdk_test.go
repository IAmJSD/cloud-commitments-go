package recommendations_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LeanerCloud/cloud-commitments-go/pkg/common"
	"github.com/LeanerCloud/cloud-commitments-go/pkg/scorer"
	"github.com/LeanerCloud/cloud-commitments-go/providers/aws/recommendations"
)

// breakEvenFixtureDerived is what common.BreakEven derives from the fixture's
// costs: 1200 / (100 + 1200/12) = 6 months. Every AWS figure below differs
// from it so a test can tell which one the pipeline used.
const breakEvenFixtureDerived = 6.0

// breakEvenSDKRecommendations runs a constructed (not recorded) CE
// GetReservationPurchaseRecommendation response through the real SDK
// deserializer and the RI parser. breakEven nil omits EstimatedBreakEvenInMonths.
func breakEvenSDKRecommendations(t *testing.T, breakEven *string) ([]common.Recommendation, error) {
	t.Helper()
	detail := map[string]any{
		"AccountId":                              "fixture-account",
		"CurrencyCode":                           "USD",
		"RecommendedNumberOfInstancesToPurchase": "2",
		"UpfrontCost":                            "1200",
		"RecurringStandardMonthlyCost":           "50",
		"EstimatedMonthlySavingsAmount":          "100",
		"EstimatedMonthlySavingsPercentage":      "40",
		"EstimatedMonthlyOnDemandCost":           "250",
		"InstanceDetails": map[string]any{"RDSInstanceDetails": map[string]any{
			"InstanceType": "db.m5.large", "DatabaseEngine": "MySQL", "Region": "us-east-1"}},
	}
	if breakEven != nil {
		detail["EstimatedBreakEvenInMonths"] = *breakEven
	}
	fixture := &currencySDKFixture{t: t, detail: detail, payment: "partial-upfront", term: "1yr"}
	client := recommendations.NewClient(&aws.Config{Region: "us-east-1", HTTPClient: fixture, RetryMaxAttempts: 1,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "synthetic", SecretAccessKey: "synthetic"}, nil
		})})
	got, err := client.GetRecommendations(context.Background(), &common.RecommendationParams{
		Service: common.ServiceRDS, Term: "1yr", PaymentOption: "partial-upfront", LookbackPeriod: "7d"})
	assert.Equal(t, 1, fixture.calls)
	return got, err
}

// TestRIBreakEvenFromCostExplorer is the regression test for go#390: the RI
// parser must carry CE's EstimatedBreakEvenInMonths into BreakEvenMonths so
// common.BreakEven prefers AWS's figure, and must never fabricate a value for
// an absent or corrupt one.
func TestRIBreakEvenFromCostExplorer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		value     *string
		wantField float64
		wantUsed  float64
	}{
		{"fractional", aws.String("7.5"), 7.5, 7.5},
		{"integer", aws.String("4"), 4, 4},
		{"absent falls back to derivation", nil, 0, breakEvenFixtureDerived},
		{"zero stays unknown and falls back to derivation", aws.String("0"), 0, breakEvenFixtureDerived},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := breakEvenSDKRecommendations(t, tc.value)
			require.NoError(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, tc.wantField, got[0].BreakEvenMonths)
			months, known := common.BreakEven(got[0])
			assert.True(t, known)
			assert.InDelta(t, tc.wantUsed, months, 1e-9)
		})
	}
}

func TestRIBreakEvenFromCostExplorerRejectsCorruptValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "seven", "7.5 months", "NaN", "+Inf", "-1"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			got, err := breakEvenSDKRecommendations(t, aws.String(value))
			require.Error(t, err)
			assert.ErrorContains(t, err, "EstimatedBreakEvenInMonths")
			assert.Empty(t, got, "a corrupt break-even must drop the detail, not become 0")
		})
	}
}

// TestRIBreakEvenScorerUsesCostExplorerValue checks --max-break-even-months
// against AWS's figure, not the 6-month derivation, in both directions.
func TestRIBreakEvenScorerUsesCostExplorerValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value string
		max         int
		pass        bool
	}{
		{"AWS above limit, derivation below", "7.5", 7, false},
		{"AWS below limit, derivation above", "4.2", 5, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := breakEvenSDKRecommendations(t, aws.String(tc.value))
			require.NoError(t, err)
			result := scorer.Score(got, scorer.Config{MaxBreakEvenMonths: tc.max})
			if !tc.pass {
				require.Len(t, result.Filtered, 1)
				assert.Contains(t, result.Filtered[0].FilterReason, "break-even "+tc.value+" months exceeds")
				return
			}
			require.Len(t, result.Passed, 1)
			assert.Equal(t, 4.2, result.Passed[0].BreakEvenMonths)
		})
	}
}

// Savings Plans recommendation details have no break-even field in the CE API
// (SavingsPlansPurchaseRecommendationDetail), so an SP rec keeps the derivation.
func TestSPBreakEvenStaysDerived(t *testing.T) {
	t.Parallel()
	got, err := currencySDKRecommendation(t, true, "partial-upfront", "1yr", aws.String("USD"), aws.String("1200"), aws.String("1"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Zero(t, got[0].BreakEvenMonths)
}
