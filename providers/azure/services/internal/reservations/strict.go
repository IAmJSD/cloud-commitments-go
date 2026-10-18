package reservations

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/reservations/armreservations"

	"github.com/LeanerCloud/cloud-commitments-go/pkg/common"
)

// StrictLabels name the service and reserved resource in StrictCommitment
// errors, e.g. {Service: "compute", Resource: "VM"}.
type StrictLabels struct {
	Service  string
	Resource string
}

// StrictCommitment validates and converts one reservation, failing closed
// instead of dropping or zero-counting incomplete inventory. It returns a nil
// commitment for reservations of another type.
func StrictCommitment(r *armreservations.ReservationResponse, subscriptionID string, service common.ServiceType, wantType armreservations.ReservedResourceType, labels StrictLabels, now time.Time) (*common.Commitment, error) {
	// filterAppliedReservations passes nil and property-less rows through;
	// dropping them would hide a possibly just-bought reservation.
	if r == nil || r.Properties == nil || r.Properties.ReservedResourceType == nil {
		return nil, fmt.Errorf("%s: cannot classify reservation %s in subscription %q: missing properties or reservedResourceType",
			labels.Service, reservationLabel(r), subscriptionID)
	}
	commitment := CommitmentFromReservation(r, subscriptionID, service, wantType, now)
	if commitment == nil {
		return nil, nil
	}
	owned, err := reservationOwnedBy(r.Properties, commitment.CommitmentID, subscriptionID, labels)
	if err != nil {
		return nil, err
	}
	if !owned {
		log.Printf("%s: skipping Shared %s reservation %q: billed to %q, not subscription %q",
			labels.Service, labels.Resource, commitment.CommitmentID, *r.Properties.BillingScopeID, subscriptionID)
		return nil, nil
	}
	if err := validateReservationExpiry(r.Properties, commitment.CommitmentID, subscriptionID, labels, now); err != nil {
		return nil, err
	}
	if !isTerminalCommitmentState(commitment.State) {
		if err := validateLiveReservationFields(r, commitment.CommitmentID, subscriptionID, labels); err != nil {
			return nil, err
		}
	}
	return commitment, nil
}

func reservationLabel(r *armreservations.ReservationResponse) string {
	if r == nil || r.ID == nil {
		return "(no id)"
	}
	return fmt.Sprintf("%q", *r.ID)
}

// isTerminalCommitmentState reports lifecycle states that no longer count as
// coverage. It works on the converted state so a Succeeded reservation with a
// past expiry is terminal too.
func isTerminalCommitmentState(s common.CommitmentState) bool {
	switch s {
	case common.CommitmentStateCanceled, common.CommitmentStateExpired,
		common.CommitmentStateFailed, common.CommitmentStateRetired:
		return true
	}
	return false
}

// validateLiveReservationFields rejects a live reservation missing a field
// the duplicate guard keys or sizes on: a blank SKU or location never matches a
// recommendation, a missing quantity counts as zero capacity, and a missing
// purchase date makes a fresh purchase look old. Erroring (not skipping) is
// deliberate: the row may be the reservation bought an hour ago.
func validateLiveReservationFields(r *armreservations.ReservationResponse, reservationID, subscriptionID string, labels StrictLabels) error {
	var missing []string
	if r.SKU == nil || r.SKU.Name == nil || strings.TrimSpace(*r.SKU.Name) == "" {
		missing = append(missing, "sku.name")
	}
	if r.Location == nil || strings.TrimSpace(*r.Location) == "" {
		missing = append(missing, "location")
	}
	if r.Properties.Quantity == nil || *r.Properties.Quantity <= 0 {
		missing = append(missing, "quantity")
	}
	if r.Properties.PurchaseDate == nil {
		missing = append(missing, "purchaseDate")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: incomplete live %s reservation %q in subscription %q: missing or invalid %s",
			labels.Service, labels.Resource, reservationID, subscriptionID, strings.Join(missing, ", "))
	}
	return nil
}

// reservationOwnedBy decides whether a reservation from the applied list is
// this subscription's commitment. A Single reservation was already matched to
// the subscription by filterAppliedReservations. A Shared reservation applies
// across its billing context, so it is attributed only to the subscription its
// billingScopeId names: CUDly purchases under /subscriptions/{sub}, so the
// buying subscription keeps its duplicate guard and no other subscription is
// credited with the same quantity (issue #293). A Shared reservation whose
// purchaser cannot be identified, and any other scope type, fails closed: it
// may be the reservation bought an hour ago.
func reservationOwnedBy(props *armreservations.Properties, reservationID, subscriptionID string, labels StrictLabels) (bool, error) {
	if props.AppliedScopeType == nil {
		return false, fmt.Errorf("%s: cannot attribute %s reservation %q to subscription %q: missing applied scope type", labels.Service, labels.Resource, reservationID, subscriptionID)
	}
	switch *props.AppliedScopeType {
	case armreservations.AppliedScopeTypeSingle:
		return true, nil
	case armreservations.AppliedScopeTypeShared:
		billingSubscription, ok := billingSubscriptionID(props.BillingScopeID)
		if !ok {
			return false, fmt.Errorf("%s: cannot attribute %s reservation %q to subscription %q: Shared scope without a subscription billingScopeId", labels.Service, labels.Resource, reservationID, subscriptionID)
		}
		return strings.EqualFold(billingSubscription, subscriptionID), nil
	default:
		return false, fmt.Errorf("%s: cannot attribute %s reservation %q to subscription %q: unsupported applied scope type %q", labels.Service, labels.Resource, reservationID, subscriptionID, *props.AppliedScopeType)
	}
}

// billingSubscriptionID extracts {sub} from a billingScopeId of exactly
// /subscriptions/{sub}. Anything else (nil, blank, another resource path) is
// not a purchasing subscription.
func billingSubscriptionID(scope *string) (string, bool) {
	if scope == nil {
		return "", false
	}
	segments := strings.Split(*scope, "/")
	if len(segments) != 3 || segments[0] != "" || !strings.EqualFold(segments[1], "subscriptions") {
		return "", false
	}
	sub := segments[2]
	if sub == "" || strings.IndexFunc(sub, unicode.IsSpace) >= 0 {
		return "", false
	}
	return sub, true
}

func validateReservationExpiry(props *armreservations.Properties, reservationID, subscriptionID string, labels StrictLabels, now time.Time) error {
	if props.ProvisioningState != nil && *props.ProvisioningState == armreservations.ProvisioningStateSucceeded &&
		props.ExpiryDate != nil && props.ExpiryDate.UTC().Format(time.DateOnly) == now.UTC().Format(time.DateOnly) {
		return fmt.Errorf("%s: cannot determine lifecycle for %s reservation %q in subscription %q: reservations API date-only expiryDate %s falls on the current UTC date",
			labels.Service, labels.Resource, reservationID, subscriptionID, props.ExpiryDate.UTC().Format(time.DateOnly))
	}
	return nil
}
