package recommendations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LeanerCloud/cloud-commitments-go/pkg/common"
)

func TestSPAccountScope(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"", "LINKED"}, {"linked", "LINKED"}, {"payer", "PAYER"}, {"PAYER", "PAYER"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			fixture := &spSDKFixture{respond: func(spSDKRequest) (map[string]any, error) { return spSDKPage(""), nil }}
			params := spSDKParams(common.ServiceSavingsPlansCompute)
			params.SPAccountScope = tc.in
			_, err := spSDKClient(fixture).GetRecommendations(context.Background(), params)
			require.NoError(t, err)
			require.Len(t, fixture.requests, 1)
			require.Equal(t, tc.want, fixture.requests[0].AccountScope)
		})
	}
}

func TestSPAccountScopeRejectsUnknown(t *testing.T) {
	t.Parallel()
	fixture := &spSDKFixture{respond: func(spSDKRequest) (map[string]any, error) { return spSDKPage(""), nil }}
	params := spSDKParams(common.ServiceSavingsPlansCompute)
	params.SPAccountScope = "org"
	_, err := spSDKClient(fixture).GetRecommendations(context.Background(), params)
	require.ErrorContains(t, err, "invalid Savings Plans account scope")
	require.Empty(t, fixture.requests)
}
