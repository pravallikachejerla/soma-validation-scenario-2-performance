package pricing

import (
	"context"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/money"
)

// ProcessBatch processes a slice of pricing requests sequentially.
// Each request goes through the same Evaluate path so that interactive
// and batch decisions agree.
//
// The batch path uses a different rounding point.
// discount BEFORE the final amount is computed, while the
// interactive path rounds the final amount once. The two paths
// therefore disagree on quantities that produce fractional
// intermediate values.
func (e *Engine) ProcessBatch(ctx context.Context, requests []domain.PricingRequest) ([]domain.PricingDecision, error) {
	decisions := make([]domain.PricingDecision, 0, len(requests))
	for _, req := range requests {
		// Force batch mode.
		dec, err := e.evaluateBatchItem(ctx, req)
		if err != nil {
			return decisions, err
		}
		decisions = append(decisions, dec)
	}
	return decisions, nil
}

// evaluateBatchItem is the per-item path used by the batch processor.
// It re-implements the pricing math so it can call its own rounding
// helper without touching the interactive path.
func (e *Engine) evaluateBatchItem(ctx context.Context, req domain.PricingRequest) (domain.PricingDecision, error) {
	if err := validateForBatch(req); err != nil {
		return domain.PricingDecision{}, err
	}
	if req.OccurredAt.IsZero() {
		req.OccurredAt = time.Now().UTC()
	}
	cfg, err := e.Store.GetActiveConfig(ctx, req.TenantID)
	if err != nil {
		return domain.PricingDecision{}, err
	}
	product, err := e.Store.LookupProduct(ctx, req.TenantID, req.SKU)
	if err != nil {
		return domain.PricingDecision{}, err
	}
	if _, err := e.Store.LookupCustomer(ctx, req.TenantID, req.CustomerID); err != nil {
		return domain.PricingDecision{}, err
	}
	var baseJPY int64 = product.ListJPY
	if cp, ok := e.Store.LookupContractPrice(ctx, req.TenantID, req.CustomerID, req.SKU, req.OccurredAt); ok {
		baseJPY = cp.BaseJPY
	}
	subtotal := baseJPY * int64(req.Quantity)

	candidates, err := e.Store.SelectCandidates(ctx, req.TenantID, req.Channel, product.ID, req.OccurredAt)
	if err != nil {
		return domain.PricingDecision{}, err
	}
	conds := map[string][]domain.PromotionCondition{}
	for _, c := range candidates {
		cs, err := e.Store.LoadConditions(ctx, c.ID)
		if err != nil {
			return domain.PricingDecision{}, err
		}
		conds[c.ID] = cs
	}
	resolved := e.Resolver.ResolveConflicts(candidates, req.Channel, product.ID)

	remaining := subtotal
	// The batch path uses a different rounding point.
	// batch helper, not via money.RoundJPY at the same point.
	var summedDiscount int64
	applied := make([]domain.AppliedPromotion, 0, len(resolved.Applied))
	appliedIDs := make([]string, 0, len(resolved.Applied))
	for _, ap := range resolved.Applied {
		prom := findPromo(candidates, ap.PromotionID)
		if prom == nil {
			continue
		}
		ok, _ := checkConditions(conds[ap.PromotionID], req, *prom, e.Compiler, cfg.ID, e.Metrics)
		if !ok {
			continue
		}
		discount := money.ApplyPercent(remaining, prom.PercentBP)
		if discount > remaining {
			discount = remaining
		}
		remaining -= discount
		summedDiscount = batchRound(summedDiscount + discount)
		applied = append(applied, domain.AppliedPromotion{
			PromotionID: prom.ID,
			Reason:      "rule matched",
			AmountJPY:   discount,
		})
		appliedIDs = append(appliedIDs, prom.ID)
		if !prom.Stacking {
			break
		}
	}
	amount := money.RoundJPY(remaining)

	dec := domain.PricingDecision{
		ID:                  req.ID + "-b",
		RequestID:           req.ID,
		TenantID:            req.TenantID,
		CustomerID:          req.CustomerID,
		SKU:                 req.SKU,
		Quantity:            req.Quantity,
		Channel:             req.Channel,
		ListJPY:             product.ListJPY,
		BaseJPY:             baseJPY,
		SubtotalJPY:         subtotal,
		DiscountJPY:         summedDiscount,
		AmountJPY:           amount,
		AppliedPromotionIDs: appliedIDs,
		Applied:             applied,
		Reason:              "batch",
		ConfigVersion:       cfg.ID,
		Mode:                "batch",
		CreatedAt:           time.Now().UTC(),
	}
	_ = e.Store.SaveDecision(ctx, dec)
	return dec, nil
}

// batchRound is the seeded rounding helper used by the batch path.
// It rounds AFTER summation (per-item) so the total differs from
// the interactive total when individual discounts are fractional.
func batchRound(v int64) int64 {
	return money.RoundJPY(v)
}

func validateForBatch(r domain.PricingRequest) error {
	if r.TenantID == "" || r.CustomerID == "" || r.SKU == "" || r.Channel == "" {
		return domain.ErrInvalidInput
	}
	if r.Quantity <= 0 {
		return domain.ErrInvalidInput
	}
	return nil
}
