// Package validation provides bounded input validation helpers.
package validation

import (
	"errors"
	"strings"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// MaxRequestBodyBytes bounds request payload size (defence in depth).
const MaxRequestBodyBytes = 64 * 1024

// ValidatePricingRequest checks that the user-supplied request carries
// the required fields. Empty values are rejected with a domain error.
func ValidatePricingRequest(r domain.PricingRequest) error {
	if strings.TrimSpace(r.TenantID) == "" {
		return domain.ErrInvalidInput
	}
	if strings.TrimSpace(r.CustomerID) == "" {
		return domain.ErrInvalidInput
	}
	if strings.TrimSpace(r.SKU) == "" {
		return domain.ErrInvalidInput
	}
	if strings.TrimSpace(r.Channel) == "" {
		return domain.ErrInvalidInput
	}
	if r.Quantity <= 0 {
		return domain.ErrInvalidInput
	}
	return nil
}

// IsValidChannel accepts a bounded set of known channel identifiers.
func IsValidChannel(c string) bool {
	switch c {
	case "web", "store", "partner", "api", "wholesale":
		return true
	}
	return false
}

// ClampLimit returns a sane page size.
func ClampLimit(n, def, max int) int {
	if n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// MustNotContainControl rejects control characters in free-form text.
func MustNotContainControl(s string) error {
	for _, r := range s {
		if r < 0x20 && r != '\t' {
			return errors.New("control character not allowed")
		}
	}
	return nil
}
