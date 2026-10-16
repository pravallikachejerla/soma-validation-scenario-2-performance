package cache_test

// Tenant / configuration isolation through the pricing engine with the cache
// ENABLED.
//
// Two engines, each over its own store, share ONE cache. Both tenants quote
// the same (customer, sku, qty, channel, date) but have different promotions,
// so a correct cache must keep their decisions apart. (A single MemoryStore
// cannot hold the same customer id for two tenants, so the shared cache is the
// isolation boundary exercised here.)
//
// On the frozen baseline the key omits tenant and config version (finding F6),
// so the baseline run characterises the leak instead of failing. Candidates
// that claim to fix F6 run with SOMA_ENFORCE_ISOLATION=1 and must pass.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/pricing"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/ruleengine"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"
)

func enforce() bool { return os.Getenv("SOMA_ENFORCE_ISOLATION") == "1" }

var quoteAt = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func promo(tenant string, bp int) domain.Promotion {
	return domain.Promotion{
		ID: "promo-" + tenant, TenantID: tenant, Name: "Sale", Channel: "web",
		ProductScope: "wildcard", Priority: 1, PercentBP: bp,
		ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}
}

// tenantEngine builds a one-tenant engine over the given shared cache. Every
// tenant uses the SAME customer id, sku and config id.
func tenantEngine(tenant string, bp int, pc *cache.PricingCache) (*pricing.Engine, *storage.MemoryStore) {
	mem := storage.NewMemoryStore()
	mem.PutTenant(domain.Tenant{ID: tenant, Name: tenant})
	mem.PutProduct(domain.Product{ID: "p-" + tenant, TenantID: tenant, SKU: "SKU-1", Name: "Widget", ListJPY: 10000})
	mem.PutCustomer(domain.Customer{ID: "cust-1", TenantID: tenant, Name: "Acme"})
	mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-1", TenantID: tenant, Active: true})
	mem.SetActiveConfig(tenant, "cfg-1")
	mem.PutPromotion(promo(tenant, bp), nil)
	m := observability.NewMetrics(prometheus.NewRegistry())
	return pricing.New(mem, pc, ruleengine.NewCompiler(), m), mem
}

func quote(t *testing.T, e *pricing.Engine, tenant string) domain.PricingDecision {
	t.Helper()
	d, err := e.Evaluate(context.Background(), domain.PricingRequest{
		ID: "r-" + tenant, TenantID: tenant, CustomerID: "cust-1", SKU: "SKU-1",
		Quantity: 2, Channel: "web", OccurredAt: quoteAt,
	})
	if err != nil {
		t.Fatalf("evaluate %s: %v", tenant, err)
	}
	return d
}

// leak reports a failed isolation expectation: fatal when enforcing, a logged
// characterisation of baseline defect F6 otherwise.
func leak(t *testing.T, format string, args ...any) {
	t.Helper()
	if enforce() {
		t.Fatalf(format, args...)
	}
	t.Logf("BASELINE DEFECT F6 (characterised): "+format, args...)
}

func TestTenantIsolationThroughEngine(t *testing.T) {
	// 2 x 10000 JPY: tenant-a gets 10% off (18000), tenant-b 20% off (16000).
	const wantA, wantB = int64(18000), int64(16000)

	for _, order := range [][2]string{{"tenant-a", "tenant-b"}, {"tenant-b", "tenant-a"}} {
		shared := cache.New(64, time.Minute)
		engA, _ := tenantEngine("tenant-a", 1000, shared)
		engB, _ := tenantEngine("tenant-b", 2000, shared)
		engines := map[string]*pricing.Engine{"tenant-a": engA, "tenant-b": engB}
		want := map[string]int64{"tenant-a": wantA, "tenant-b": wantB}

		for _, tn := range order {
			d := quote(t, engines[tn], tn)
			if d.AmountJPY != want[tn] || d.TenantID != tn {
				leak(t, "order %v: %s was served amount=%d tenant=%q, want amount=%d tenant=%q",
					order, tn, d.AmountJPY, d.TenantID, want[tn], tn)
			}
		}
		// The first tenant priced always sees its own value, so the test is
		// not vacuous even on the baseline.
		first := quote(t, engines[order[0]], order[0])
		if first.AmountJPY != want[order[0]] {
			t.Fatalf("first tenant %s priced %d, want %d", order[0], first.AmountJPY, want[order[0]])
		}
	}
}

func TestCacheStillServesSameTenantRepeats(t *testing.T) {
	pc := cache.New(64, time.Minute)
	e, _ := tenantEngine("tenant-a", 1000, pc)
	first := quote(t, e, "tenant-a")
	second := quote(t, e, "tenant-a")
	if first.ID != second.ID {
		t.Fatalf("repeat quote was not served from the cache (ids %s vs %s)", first.ID, second.ID)
	}
	if pc.Size() != 1 {
		t.Fatalf("expected exactly one cached entry, got %d", pc.Size())
	}
}

func TestConfigVersionIsolationThroughEngine(t *testing.T) {
	pc := cache.New(64, time.Minute)
	e, mem := tenantEngine("tenant-a", 1000, pc)
	before := quote(t, e, "tenant-a")
	if before.AmountJPY != 18000 {
		t.Fatalf("setup: want 18000, got %d", before.AmountJPY)
	}

	// Roll the tenant forward to a new configuration whose promotion is 20%.
	mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-2", TenantID: "tenant-a", Active: true})
	mem.SetActiveConfig("tenant-a", "cfg-2")
	mem.PutPromotion(promo("tenant-a", 2000), nil)

	after := quote(t, e, "tenant-a")
	if after.ConfigVersion != "cfg-2" || after.AmountJPY != 16000 {
		leak(t, "stale decision served after config roll-forward: config=%q amount=%d, want config=cfg-2 amount=16000",
			after.ConfigVersion, after.AmountJPY)
	}
}
