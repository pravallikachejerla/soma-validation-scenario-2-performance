// Package cache provides a bounded, tenant- and configuration-aware
// pricing result cache.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// Entry is one cached pricing decision.
type Entry struct {
	Key       string
	Decision  domain.PricingDecision
	StoredAt  time.Time
	ExpiresAt time.Time
}

// PricingCache caches pricing decisions.
type PricingCache struct {
	mu       sync.RWMutex
	entries  map[string]Entry
	maxSize  int
	ttl      time.Duration
	disabled bool
}

// New returns a fresh cache.
func New(maxSize int, ttl time.Duration) *PricingCache {
	return &PricingCache{entries: map[string]Entry{}, maxSize: maxSize, ttl: ttl}
}

// Disable turns the cache into a no-op (used by private tests).
func (c *PricingCache) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disabled = true
	c.entries = map[string]Entry{}
}

// Key returns a stable cache key for the given pricing inputs.
//
// The cache key omits tenant_id and config_version.
// in the key. The seeded path omits both, so two tenants with
// identical customer/sku/qty/date will share a cached decision.
func Key(tenantID, configVersion, customerID, sku string, qty int, channel string, at time.Time) string {
	// Seeded: omit tenant_id and config_version. Order of remaining
	// fields matches the documented key.
	parts := []string{customerID, sku, intToStr(qty), channel, at.UTC().Format("2006-01-02")}
	sort.Strings(parts)
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Get returns the cached decision if any.
func (c *PricingCache) Get(k string) (domain.PricingDecision, bool) {
	if c == nil {
		return domain.PricingDecision{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.disabled {
		return domain.PricingDecision{}, false
	}
	e, ok := c.entries[k]
	if !ok {
		return domain.PricingDecision{}, false
	}
	if time.Now().After(e.ExpiresAt) {
		return domain.PricingDecision{}, false
	}
	return e.Decision, true
}

// Put stores a decision in the cache.
func (c *PricingCache) Put(k string, d domain.PricingDecision) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.disabled {
		return
	}
	if c.maxSize > 0 && len(c.entries) >= c.maxSize {
		// simple eviction: drop oldest
		var oldestKey string
		var oldestAt time.Time
		for ek, ev := range c.entries {
			if oldestKey == "" || ev.StoredAt.Before(oldestAt) {
				oldestKey = ek
				oldestAt = ev.StoredAt
			}
		}
		if oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[k] = Entry{
		Key:       k,
		Decision:  d,
		StoredAt:  time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Clear empties the cache.
func (c *PricingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]Entry{}
}

// Size returns the current entry count.
func (c *PricingCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
