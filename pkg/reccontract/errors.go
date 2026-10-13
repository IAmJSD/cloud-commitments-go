package reccontract

import (
	"errors"
	"fmt"

	"github.com/LeanerCloud/cloud-commitments-go/pkg/common"
)

// ErrUnavailable is matched by errors.Is for any *UnavailableError.
var ErrUnavailable = errors.New("service unavailable")

// UnavailableError reports that a service cannot be recommended through the
// provider's API, with the reason the provider declared.
type UnavailableError struct {
	Provider common.ProviderType
	Service  common.ServiceType
	Reason   string
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("%s recommendations for service %q are unavailable: %s", e.Provider, e.Service, e.Reason)
}

// Is makes errors.Is(err, ErrUnavailable) true for every UnavailableError.
func (e *UnavailableError) Is(target error) bool {
	return target == ErrUnavailable
}

// CheckAvailable returns an *UnavailableError when the service is declared
// unavailable in c, and nil otherwise. It performs no I/O.
func CheckAvailable(c Capabilities, s common.ServiceType) error {
	reason, unavailable := c.Unavailable[s]
	if !unavailable {
		return nil
	}
	return &UnavailableError{Provider: c.Provider, Service: s, Reason: reason}
}
