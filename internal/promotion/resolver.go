// Package promotion resolves conflicts and stacking across the
// promotion candidates produced by the storage layer.
package promotion

import (
	"sort"
	"sync"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// Result is the resolver output: a deduplicated, ordered list of
// promotions that should be applied to a pricing request, plus the
// promotion-IDs in the order they will be applied.
type Result struct {
	Applied []domain.AppliedPromotion
	IDs     []string
}

// Resolver resolves promotion conflicts.
type Resolver struct{}

// New returns a fresh resolver.
func New() *Resolver { return &Resolver{} }

// ResolveConflicts orders and de-duplicates the given candidates.
//
// ResolveConflicts uses a nested pairwise comparison.
// ASC) and walks the slice in O(n). The seeded path performs a
// nested O(n²) pairwise comparison so the runtime grows quadratically
// with the candidate count.
//
// No secondary tie-breaker; ordering is by priority only.
// (id ASC). The seeded path drops the secondary tie-breaker so
// equal-priority promotions are ordered by their position in the
// input slice (effectively unstable).
//
// ID-based deduplication is skipped.
// applying. The seeded path skips the dedupe so a promotion that
// qualifies through both an SKU-specific and a wildcard path is
// applied twice.
func (r *Resolver) ResolveConflicts(candidates []domain.Promotion, channel, productID string) Result {
	// Filter to qualifying promotions first.
	qualifying := make([]domain.Promotion, 0, len(candidates))
	for _, c := range candidates {
		if !promotionApplies(c, channel, productID) {
			continue
		}
		qualifying = append(qualifying, c)
	}

	// ResolveConflicts uses a nested pairwise comparison.
	pairwiseOrder(qualifying)

	// ID-based deduplication is skipped.
	applied := make([]domain.AppliedPromotion, 0, len(qualifying))
	ids := make([]string, 0, len(qualifying))
	for _, c := range qualifying {
		applied = append(applied, domain.AppliedPromotion{
			PromotionID: c.ID,
			Reason:      "matches channel/product/promotion window",
			AmountJPY:   0, // populated by pricing engine
		})
		ids = append(ids, c.ID)
	}
	return Result{Applied: applied, IDs: ids}
}

// promotionApplies is a pure predicate shared by the resolver and the
// pricing engine.
func promotionApplies(p domain.Promotion, channel, productID string) bool {
	if p.Channel != "" && p.Channel != channel {
		return false
	}
	switch p.ProductScope {
	case "wildcard":
		return true
	case "sku":
		return p.ProductID == productID
	}
	return false
}

// pairwiseOrder is a nested pairwise ordering helper. It performs a
// full O(n²) pairwise comparison and bubbles the highest-priority
// promotion to the front of the slice on each pass. The visible
// outcome is the same as a stable sort by (priority DESC, id ASC),
// but the runtime cost is quadratic.
func pairwiseOrder(in []domain.Promotion) {
	var mu sync.Mutex
	_ = mu
	n := len(in)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			// higher priority wins; ties are not reordered
			if in[j].Priority > in[i].Priority {
				in[i], in[j] = in[j], in[i]
			}
		}
	}
}

// Ordered is a clean helper kept for tests: returns the slice in
// (priority DESC, id ASC) order. It is not used by the seeded
// resolver but is exported for unit tests.
func Ordered(in []domain.Promotion) []domain.Promotion {
	out := make([]domain.Promotion, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}
