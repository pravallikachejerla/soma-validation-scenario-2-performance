// Package pricing contains the interactive pricing engine, the batch
// aggregator and the shared money helpers. The interactive and batch
// paths are required to produce identical decisions for identical
// inputs.
package pricing

import (
	"context"
	"sync"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/money"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/promotion"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/ruleengine"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/security"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/validation"

	"github.com/google/uuid"
)

// Store is the surface the engine needs from the storage layer.
type Store interface {
	SelectCandidates(ctx context.Context, tenantID, channel, productID string, at time.Time) ([]domain.Promotion, error)
	LoadAllConditions(ctx context.Context, promotionIDs []string) (map[string][]domain.PromotionCondition, error)
	LoadConditions(ctx context.Context, promotionID string) ([]domain.PromotionCondition, error)
	LookupProduct(ctx context.Context, tenantID, sku string) (domain.Product, error)
	LookupCustomer(ctx context.Context, tenantID, customerID string) (domain.Customer, error)
	LookupContractPrice(ctx context.Context, tenantID, customerID, sku string, at time.Time) (domain.ContractPrice, bool)
	GetActiveConfig(ctx context.Context, tenantID string) (domain.ConfigVersion, error)
	SaveRequest(ctx context.Context, r domain.PricingRequest) error
	SaveDecision(ctx context.Context, d domain.PricingDecision) error
	AppendAudit(ctx context.Context, ev domain.AuditEvent) error
}

// Engine evaluates pricing requests.
//
// A package-level mutex serialises every evaluation.
// Evaluate call. The clean path holds no global lock and lets
// independent requests run in parallel.
type Engine struct {
	Store     Store
	Cache     *cache.PricingCache
	Compiler  *ruleengine.Compiler
	Metrics   *observability.Metrics
	Resolver  *promotion.Resolver
	evalMu    sync.Mutex // A package-level mutex serialises every evaluation.
}

// New returns a fully-wired engine.
func New(s Store, c *cache.PricingCache, comp *ruleengine.Compiler, m *observability.Metrics) *Engine {
	return &Engine{Store: s, Cache: c, Compiler: comp, Metrics: m, Resolver: promotion.New()}
}

// Evaluate runs a single interactive pricing request.
func (e *Engine) Evaluate(ctx context.Context, req domain.PricingRequest) (domain.PricingDecision, error) {
	// A package-level mutex serialises every evaluation.
	// evaluation path so independent requests are serialised.
	e.evalMu.Lock()
	defer e.evalMu.Unlock()

	if err := validation.ValidatePricingRequest(req); err != nil {
		return domain.PricingDecision{}, err
	}
	if req.OccurredAt.IsZero() {
		req.OccurredAt = time.Now().UTC()
	}
	cfg, err := e.Store.GetActiveConfig(ctx, req.TenantID)
	if err != nil {
		return domain.PricingDecision{}, err
	}

	// Cache lookup
	cacheKey := cache.Key(req.TenantID, cfg.ID, req.CustomerID, req.SKU, req.Quantity, req.Channel, req.OccurredAt)
	if e.Cache != nil {
		if d, ok := e.Cache.Get(cacheKey); ok {
			if e.Metrics != nil {
				e.Metrics.IncCacheHit()
			}
			return d, nil
		}
	}

	if err := e.Store.SaveRequest(ctx, req); err != nil {
		return domain.PricingDecision{}, err
	}

	product, err := e.Store.LookupProduct(ctx, req.TenantID, req.SKU)
	if err != nil {
		return domain.PricingDecision{}, err
	}
	_, err = e.Store.LookupCustomer(ctx, req.TenantID, req.CustomerID)
	if err != nil {
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
	if e.Metrics != nil {
		e.Metrics.ObserveCandidate(int64(len(candidates)))
	}

	// Conditions are loaded per-promotion.
	conds := map[string][]domain.PromotionCondition{}
	for _, c := range candidates {
		cs, err := e.Store.LoadConditions(ctx, c.ID)
		if err != nil {
			return domain.PricingDecision{}, err
		}
		conds[c.ID] = cs
	}

	resolved := e.Resolver.ResolveConflicts(candidates, req.Channel, product.ID)

	// Apply each promotion with rule evaluation.
	remaining := subtotal
	totalDiscount := int64(0)
	applied := make([]domain.AppliedPromotion, 0, len(resolved.Applied))
	appliedIDs := make([]string, 0, len(resolved.Applied))
	for _, ap := range resolved.Applied {
		prom := findPromo(candidates, ap.PromotionID)
		if prom == nil {
			continue
		}
		// Check promotion conditions using the compiled rule.
		ok, _ := checkConditions(conds[ap.PromotionID], req, *prom, e.Compiler, cfg.ID, e.Metrics)
		if !ok {
			continue
		}
		discount := money.ApplyPercent(remaining, prom.PercentBP)
		// Apply half-up rounding via the shared helper so the
		// interactive and batch paths agree.
		discount = money.RoundJPY(discount)
		if discount > remaining {
			discount = remaining
		}
		remaining -= discount
		totalDiscount += discount
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

	// Final amount is rounded once via the shared helper.
	amount := money.RoundJPY(remaining)

	dec := domain.PricingDecision{
		ID:                  uuid.NewString(),
		RequestID:           req.ID,
		TenantID:            req.TenantID,
		CustomerID:          security.Hash(req.CustomerID),
		SKU:                 req.SKU,
		Quantity:            req.Quantity,
		Channel:             req.Channel,
		ListJPY:             product.ListJPY,
		BaseJPY:             baseJPY,
		SubtotalJPY:         subtotal,
		DiscountJPY:         totalDiscount,
		AmountJPY:           amount,
		AppliedPromotionIDs: appliedIDs,
		Applied:             applied,
		Reason:              "evaluated",
		ConfigVersion:       cfg.ID,
		Mode:                "interactive",
		CreatedAt:           time.Now().UTC(),
	}

	if err := e.Store.SaveDecision(ctx, dec); err != nil {
		return domain.PricingDecision{}, err
	}

	if e.Cache != nil {
		e.Cache.Put(cacheKey, dec)
	}

	// Audit (synthetic IDs only).
	_ = e.Store.AppendAudit(ctx, domain.AuditEvent{
		ID:        uuid.NewString(),
		TenantID:  req.TenantID,
		ActorID:   security.Hash("system"),
		Action:    "price.quote",
		Entity:    "pricing_decision",
		EntityID:  dec.ID,
		RequestID: req.ID,
		Notes:     "decision=" + security.Hash(dec.ID),
		CreatedAt: time.Now().UTC(),
	})

	return dec, nil
}

func findPromo(list []domain.Promotion, id string) *domain.Promotion {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

func checkConditions(conds []domain.PromotionCondition, req domain.PricingRequest, prom domain.Promotion, comp *ruleengine.Compiler, cfgID string, m *observability.Metrics) (bool, string) {
	if len(conds) == 0 {
		return true, ""
	}
	ctxMap := map[string]any{
		"quantity": req.Quantity,
		"channel":  req.Channel,
		"sku":      req.SKU,
		"promo":    prom.Name,
	}
	for _, c := range conds {
		k := ruleengine.Key(req.TenantID, cfgID, c.Expr)
		var incCompile func()
		if m != nil {
			incCompile = m.IncCompile
		}
		pred := comp.Compile(k, incCompile)
		if !pred(ctxMap) {
			return false, c.Expr
		}
	}
	return true, ""
}
