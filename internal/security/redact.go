// Package security provides redaction helpers used by the logging and
// HTTP layers. Logs must never carry raw customer identifiers,
// negotiated prices or discount reasons.
package security

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash returns a SHA-256 hex digest of id, truncated to 12 chars.
// It is used to produce a synthetic-safe identifier that is stable
// across requests and that cannot be reversed to the original.
func Hash(id string) string {
	sum := sha256.Sum256([]byte(id))
	return "id-" + hex.EncodeToString(sum[:])[:12]
}

// RedactedValue is a sentinel returned for values that must be hidden.
const RedactedValue = "[redacted]"

// IsRedacted returns true if v carries the redaction sentinel.
func IsRedacted(v string) bool { return v == RedactedValue }
