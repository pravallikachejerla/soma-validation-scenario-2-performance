package pricing

import (
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// AggregateSummary summarises a batch run.
type AggregateSummary struct {
	TotalItems       int     `json:"total_items"`
	TotalAmountJPY   int64   `json:"total_amount_jpy"`
	TotalDiscountJPY int64   `json:"total_discount_jpy"`
	AppliedDistinct  int     `json:"applied_distinct_promotions"`
	AvgDiscountPct   float64 `json:"avg_discount_pct"`
}

// Aggregate computes summary statistics for a batch run.
func Aggregate(decisions []domain.PricingDecision) AggregateSummary {
	sum := AggregateSummary{TotalItems: len(decisions)}
	seen := map[string]struct{}{}
	for _, d := range decisions {
		sum.TotalAmountJPY += d.AmountJPY
		sum.TotalDiscountJPY += d.DiscountJPY
		for _, id := range d.AppliedPromotionIDs {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				sum.AppliedDistinct++
			}
		}
	}
	if sum.TotalItems > 0 {
		sum.AvgDiscountPct = float64(sum.TotalDiscountJPY) / float64(sum.TotalItems)
	}
	return sum
}
