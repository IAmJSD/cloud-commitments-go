package insurance

import "fmt"

// redactedText is what every printer and marshaler emits for a Secret.
const redactedText = "[redacted]"

// Secret holds a credential so that no printer can reach it. The value lives
// only in a closure, so reflection-based printers (fmt on unexported fields,
// spew, encoders) see a func address, never the string. The zero Secret holds
// the empty string.
type Secret struct{ reveal func() string }

// NewSecret wraps a credential.
func NewSecret(s string) Secret { return Secret{reveal: func() string { return s }} }

// Reveal returns the credential. Call it only where the value is sent, such as
// the header-set site.
func (s Secret) Reveal() string {
	if s.reveal == nil {
		return ""
	}
	return s.reveal()
}

// Format redacts the value for every fmt verb and for slog text output.
func (Secret) Format(f fmt.State, _ rune) { _, _ = fmt.Fprint(f, redactedText) }

// MarshalText redacts the value for encoding/json, encoding/xml, yaml and
// slog's JSON handler.
func (Secret) MarshalText() ([]byte, error) { return []byte(redactedText), nil }
