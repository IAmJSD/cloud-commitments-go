// Package reccontract is the provider-neutral recommendation request contract:
// what each provider declares it can honour, and the errors raised when a
// request asks for more.
package reccontract

import "github.com/LeanerCloud/cloud-commitments-go/pkg/common"

// Capabilities declares which request values a provider can honour. An empty
// slice means the field is unsupported: any non-empty request for it must fail
// loud rather than be silently ignored.
type Capabilities struct {
	Provider common.ProviderType
	// Terms are the supported commitment terms, e.g. "1yr", "3yr".
	Terms []string
	// PaymentOptions are the supported canonical payment tokens.
	PaymentOptions []string
	// Lookbacks are the supported lookback periods, e.g. "7d", "30d", "60d".
	Lookbacks []string
	// DefaultLookback fills an empty request lookback; empty when the provider
	// has no lookback input.
	DefaultLookback string
	// AccountScopes are the supported scopes, e.g. "linked", "payer".
	AccountScopes []string
	// OfferingClasses are the supported offering classes, e.g. "standard",
	// "convertible".
	OfferingClasses []string
	// SavingsPlanTypes reports whether Savings Plan type include/exclude
	// filters are meaningful for the provider.
	SavingsPlanTypes bool
	// Unavailable maps a service to the reason it cannot be recommended
	// through the provider's API. A present key makes the service unavailable
	// even when the reason is empty.
	Unavailable map[common.ServiceType]string
}
