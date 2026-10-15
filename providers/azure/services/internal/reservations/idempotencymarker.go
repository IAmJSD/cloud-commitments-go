package reservations

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// The Reservations API does not define tags on a reservation order: neither
// armreservations.PurchaseRequest nor ReservationOrderResponse (SDK v1.1.0)
// has a Tags field, so the cudly-idempotency-token tag written by
// ApplyPurchaseTags may never be stored or listed back (issue #290). Both
// models do define properties.displayName, so the idempotency token is also
// carried there, as a short deterministic marker at the end of the name, and
// FindReservationOrderByIdempotencyToken matches on either.

// idempotencyMarkerPrefix starts every display-name marker. It is
// alphanumeric so the marker stays inside Azure's [A-Za-z0-9_-] allowlist and
// contains no '-' (the separator the marker is appended with).
const idempotencyMarkerPrefix = "cudly"

// idempotencyMarkerHexLen is how many hex digits of SHA-256(token) the marker
// keeps: 64 bits, so a collision between two tokens within one tenant's order
// listing is negligible, while leaving 42 of the 64 display-name characters
// for the human-readable part.
const idempotencyMarkerHexLen = 16

// idempotencyDisplayNameMarker returns the display-name marker for token. It
// is derived from a hash rather than a token prefix because callers may supply
// tokens of any length or character set.
func idempotencyDisplayNameMarker(token string) string {
	sum := sha256.Sum256([]byte(token))
	return idempotencyMarkerPrefix + hex.EncodeToString(sum[:])[:idempotencyMarkerHexLen]
}

// displayNameCarriesToken reports whether an order's display name ends with
// the marker for token, as written by withIdempotencyMarker.
func displayNameCarriesToken(displayName, token string) bool {
	marker := idempotencyDisplayNameMarker(token)
	return displayName == marker || strings.HasSuffix(displayName, "-"+marker)
}

// withIdempotencyMarker appends token's marker to displayName, keeping the
// result within azureDisplayNameMaxLen. To make room it drops whole trailing
// '-' segments first (for BuildDisplayName output that is the random suffix,
// then the timestamp, which the marker makes redundant for uniqueness), and
// hard-truncates only a name with no separator left to drop at.
func withIdempotencyMarker(displayName, token string) string {
	marker := idempotencyDisplayNameMarker(token)
	room := azureDisplayNameMaxLen - len(marker) - 1
	name := strings.TrimRight(displayName, "-")
	for len(name) > room {
		cut := strings.LastIndexByte(name, '-')
		if cut <= 0 {
			name = name[:room]
			break
		}
		name = strings.TrimRight(name[:cut], "-")
	}
	if name == "" {
		return marker
	}
	return name + "-" + marker
}

// applyIdempotencyDisplayName rewrites body.properties.displayName to carry
// token's marker, creating the properties map when it is absent.
func applyIdempotencyDisplayName(body map[string]interface{}, token string) {
	props, ok := body["properties"].(map[string]interface{})
	if !ok {
		if _, present := body["properties"]; present {
			// Not a shape any service client builds; leave it untouched
			// rather than overwrite properties of an unknown type.
			return
		}
		props = map[string]interface{}{}
		body["properties"] = props
	}
	current, isString := props["displayName"].(string)
	if !isString {
		// Absent (or not a string, which no service client builds): the
		// marker alone becomes the display name.
		current = ""
	}
	props["displayName"] = withIdempotencyMarker(current, token)
}
