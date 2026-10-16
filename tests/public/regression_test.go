package public

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/application"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/seeddata"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"
)

// TestCompilerCacheHits verifies that the rule compiler cache is
// actually used after the first compilation. This is a regression
// test for PERF-03 (compiler cache bypass).
func TestCompilerCacheHits(t *testing.T) {
	ds := seeddata.Build(int64(seeddata.DefaultSeed), seeddata.Medium)
	mem := storage.NewMemoryStore()
	for _, tn := range ds.TenantIDs {
		mem.PutTenant(domain.Tenant{ID: tn, Name: tn})
		mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-" + tn, TenantID: tn, Active: true})
		mem.SetActiveConfig(tn, "cfg-"+tn)
	}
	for _, pr := range ds.Products {
		mem.PutProduct(pr)
	}
	for _, c := range ds.Customers {
		mem.PutCustomer(c)
	}
	conds := map[string][]domain.PromotionCondition{}
	for _, c := range ds.Conditions {
		conds[c.PromotionID] = append(conds[c.PromotionID], c)
	}
	for _, pr := range ds.Promotions {
		mem.PutPromotion(pr, conds[pr.ID])
	}
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)
	app := application.New(mem, cache.New(0, time.Minute), m, application.BuildIdentity{Commit: "regression"})

	tenant := ds.TenantIDs[0]
	sku := ds.Products[0].SKU
	cust := ds.Customers[0].ID

	// Run 10 evaluations with the same request
	for i := 0; i < 10; i++ {
		_, err := app.Engine.Evaluate(context.Background(), domain.PricingRequest{
			TenantID: tenant, CustomerID: cust, SKU: sku, Quantity: 1, Channel: "web",
			OccurredAt: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("evaluate %d: %v", i, err)
		}
	}

	hits, misses, size := app.Compiler.Stats()
	t.Logf("Compiler stats after 10 evaluations: hits=%d misses=%d size=%d", hits, misses, size)

	// After 10 evaluations with the same expression, we expect
	// at least 1 cache hit (the expression is compiled once and
	// reused on subsequent calls).
	if hits == 0 {
		t.Errorf("expected compiler cache hits > 0, got 0 (cache bypass regression)")
	}
}

// TestBatchConditionLoading verifies that condition loading uses
// a single batch query rather than N+1 per-promotion queries.
// This is a regression test for PERF-02 (N+1 condition loading).
func TestBatchConditionLoading(t *testing.T) {
	ds := seeddata.Build(int64(seeddata.DefaultSeed), seeddata.Medium)
	mem := storage.NewMemoryStore()
	for _, tn := range ds.TenantIDs {
		mem.PutTenant(domain.Tenant{ID: tn, Name: tn})
		mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-" + tn, TenantID: tn, Active: true})
		mem.SetActiveConfig(tn, "cfg-"+tn)
	}
	for _, pr := range ds.Products {
		mem.PutProduct(pr)
	}
	for _, c := range ds.Customers {
		mem.PutCustomer(c)
	}
	conds := map[string][]domain.PromotionCondition{}
	for _, c := range ds.Conditions {
		conds[c.PromotionID] = append(conds[c.PromotionID], c)
	}
	for _, pr := range ds.Promotions {
		mem.PutPromotion(pr, conds[pr.ID])
	}
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)
	app := application.New(mem, cache.New(0, time.Minute), m, application.BuildIdentity{Commit: "regression"})

	tenant := ds.TenantIDs[0]
	sku := ds.Products[0].SKU
	cust := ds.Customers[0].ID

	_, err := app.Engine.Evaluate(context.Background(), domain.PricingRequest{
		TenantID: tenant, CustomerID: cust, SKU: sku, Quantity: 1, Channel: "web",
		OccurredAt: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	// Count LoadAllConditions vs LoadConditions in the query log.
	ql := mem.QueryLog()
	var batchCount, perPromoCount int
	for _, q := range ql {
		if q == "LoadAllConditions" {
			batchCount++
		}
		if len(q) > 15 && q[:15] == "LoadConditions:" {
			perPromoCount++
		}
	}
	t.Logf("Query log: LoadAllConditions=%d LoadConditions=%d", batchCount, perPromoCount)

	// We expect at least 1 batch call and 0 per-promotion calls.
	if batchCount == 0 {
		t.Errorf("expected LoadAllConditions > 0, got 0 (N+1 regression)")
	}
	if perPromoCount > 0 {
		t.Errorf("expected LoadConditions == 0, got %d (N+1 regression)", perPromoCount)
	}
}
