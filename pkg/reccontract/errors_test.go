package reccontract

import (
	"errors"
	"fmt"
	"testing"

	"github.com/LeanerCloud/cloud-commitments-go/pkg/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAvailable(t *testing.T) {
	caps := Capabilities{
		Provider: common.ProviderGCP,
		Unavailable: map[common.ServiceType]string{
			common.ServiceRelationalDB: "console only",
			common.ServiceCache:        "",
		},
	}

	t.Run("unavailable service carries provider, service and reason", func(t *testing.T) {
		err := CheckAvailable(caps, common.ServiceRelationalDB)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrUnavailable)

		var ue *UnavailableError
		require.True(t, errors.As(err, &ue))
		assert.Equal(t, common.ProviderGCP, ue.Provider)
		assert.Equal(t, common.ServiceRelationalDB, ue.Service)
		assert.Equal(t, "console only", ue.Reason)
		assert.Contains(t, err.Error(), "console only")
	})

	t.Run("a key with an empty reason is still unavailable", func(t *testing.T) {
		assert.ErrorIs(t, CheckAvailable(caps, common.ServiceCache), ErrUnavailable)
	})

	t.Run("services not listed are available", func(t *testing.T) {
		assert.NoError(t, CheckAvailable(caps, common.ServiceCompute))
	})

	t.Run("nil Unavailable map means everything is available", func(t *testing.T) {
		assert.NoError(t, CheckAvailable(Capabilities{}, common.ServiceRelationalDB))
	})

	t.Run("survives wrapping", func(t *testing.T) {
		err := fmt.Errorf("fetch: %w", CheckAvailable(caps, common.ServiceRelationalDB))
		assert.ErrorIs(t, err, ErrUnavailable)
	})
}
