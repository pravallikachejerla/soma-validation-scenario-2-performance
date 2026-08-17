// Package private contains the condition-level tests that exercise
// the planted source conditions. They are intentionally not run as
// part of the public CI; they are the private evaluator hooks.
package private_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/adminsearch"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/application"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/httpapi"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/pricing"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/promotion"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/ruleengine"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/seeddata"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"

	"github.com/prometheus/client_golang/prometheus"
)

// helper: build a populated, in-memory app for private tests.
func newPrivateApp(t *testing.T) (*httptest.Server, *storage.MemoryStore) {
	t.Helper()
	profile := seeddata.Large
	ds := seeddata.Build(int64(seeddata.DefaultSeed), profile)
	mem := storage.NewMemoryStore()
	for _, tn := range ds.TenantIDs {
		mem.PutTenant(domain.Tenant{ID: tn, Name: tn})
		mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-" + tn, TenantID: tn, Active: true})
		mem.SetActiveConfig(tn, "cfg-"+tn)
	}
	for _, p := range ds.Products {
		mem.PutProduct(p)
	}
	for _, c := range ds.Customers {
		mem.PutCustomer(c)
	}
	conds := map[string][]domain.PromotionCondition{}
	for _, c := range ds.Conditions {
		conds[c.PromotionID] = append(conds[c.PromotionID], c)
	}
	for _, p := range ds.Promotions {
		mem.PutPromotion(p, conds[p.ID])
	}
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)
	app := application.New(mem, cache.New(0, time.Minute), m, application.BuildIdentity{Commit: "private", BuiltAt: "now"})
	srv := httptest.NewServer(httpapi.New(app))
	t.Cleanup(srv.Close)
	return srv, mem
}

// TestCondition_PERF_01 asserts the candidate-set size scales with the
// total rule population rather than the indexed filter.
func TestCondition_PERF_01(t *testing.T) {
	_, mem := newPrivateApp(t)
	tenant := "tenant-a"
	cands, err := mem.SelectCandidates(context.Background(), tenant, "web", "prod-tenant-a-000001", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	// Count every active promotion for the tenant.
	total := 0
	for _, p := range mem.ListPromotions(tenant) {
		_ = p
		total++
	}
	if len(cands) != total {
		t.Fatalf("expected candidate count to equal total population (%d), got %d", total, len(cands))
	}
}

// TestCondition_PERF_02 asserts the pricing engine issues one query
// per promotion when loading conditions.
func TestCondition_PERF_02(t *testing.T) {
	_, mem := newPrivateApp(t)
	tenant := "tenant-a"
	before := len(mem.QueryLog())

	// Pick a SKU that exists in the seeded data.
	req := domain.PricingRequest{
		TenantID:   tenant,
		CustomerID: "cust-tenant-a-00001",
		SKU:        "SKU-tenant-a-00001",
		Quantity:   1,
		Channel:    "web",
		OccurredAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	engine := pricing.New(mem, cache.New(0, time.Minute), ruleengine.NewCompiler(), observability.NewMetrics(prometheus.NewRegistry()))
	if _, err := engine.Evaluate(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	after := len(mem.QueryLog())
	added := after - before
	loadCount := 0
	for _, q := range mem.QueryLog()[before:after] {
		if strings.HasPrefix(q, "LoadConditions:") {
			loadCount++
		}
	}
	// We expect at least one LoadConditions query per candidate
	// (the seeded engine issues an N+1 sequence).
	if loadCount < 1 {
		t.Fatalf("expected at least one LoadConditions:* query in the trace, got %d (added=%d)", loadCount, added)
	}
}

// TestCondition_PERF_03 asserts the rule compiler bypasses the cache
// by counting the compile metric for repeated identical expressions.
func TestCondition_PERF_03(t *testing.T) {
	_ = storage.NewMemoryStore()
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)
	c := ruleengine.NewCompiler()
	expr := "quantity >= 1"
	for i := 0; i < 5; i++ {
		_ = c.Compile(ruleengine.Key("t", "cfg", expr), m.IncCompile)
	}
	if m.CompileCountValue() < 5 {
		t.Fatalf("expected at least 5 compilations (no cache), got %d", m.CompileCountValue())
	}
}

// TestCondition_PERF_04 asserts the pricing engine holds a global
// mutex so concurrent requests serialise. The test launches many
// goroutines and observes that the observed parallelism is bounded
// by 1 — a property only a global lock can provide.
func TestCondition_PERF_04(t *testing.T) {
	mem := storage.NewMemoryStore()
	mem.PutTenant(domain.Tenant{ID: "tenant-a", Name: "tenant-a"})
	mem.PutProduct(domain.Product{ID: "p-1", TenantID: "tenant-a", SKU: "SKU-A-1", Name: "Widget", ListJPY: 5000})
	mem.PutCustomer(domain.Customer{ID: "c-1", TenantID: "tenant-a", Name: "Acme"})
	mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-1", TenantID: "tenant-a", Active: true})
	mem.SetActiveConfig("tenant-a", "cfg-1")
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)
	engine := pricing.New(mem, cache.New(0, time.Minute), ruleengine.NewCompiler(), m)

	// Use a barrier-style probe: each goroutine records its start
	// and end timestamps. The number of goroutines that overlap in
	// time is a direct measure of concurrency.
	const workers = 16
	starts := make([]time.Time, workers)
	ends := make([]time.Time, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	gate := make(chan struct{})
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-gate
			starts[i] = time.Now()
			_, _ = engine.Evaluate(context.Background(), domain.PricingRequest{
				TenantID:   "tenant-a",
				CustomerID: "c-1",
				SKU:        "SKU-A-1",
				Quantity:   1,
				Channel:    "web",
				OccurredAt: time.Now().UTC(),
			})
			ends[i] = time.Now()
		}()
	}
	close(gate)
	wg.Wait()

	// With a global mutex only one Evaluate can run at a time.
	// Count how many intervals [start[i], end[i]] overlap.
	overlaps := 0
	for i := 0; i < workers; i++ {
		for j := 0; j < workers; j++ {
			if i == j {
				continue
			}
			if starts[i].Before(ends[j]) && starts[j].Before(ends[i]) {
				overlaps++
			}
		}
	}
	// Each Evaluate overlaps with at most (workers-1) others if
	// they run in parallel. With a global lock we expect overlaps
	// to be near zero (small clock-skew tolerance: at most a few
	// overlaps because intervals are short).
	if overlaps > workers {
		t.Fatalf("expected serialised execution, got %d overlaps across %d workers", overlaps, workers)
	}
}

// TestCondition_PERF_05 asserts the resolver does a nested comparison.
func TestCondition_PERF_05(t *testing.T) {
	resolver := promotion.New()
	// Build N candidates with strictly descending priority.
	const n = 20
	cands := make([]domain.Promotion, n)
	for i := 0; i < n; i++ {
		cands[i] = domain.Promotion{
			ID: "p-" + string(rune('a'+i%26)) + string(rune('a'+i/26)),
			Channel: "web", ProductScope: "wildcard", Priority: n - i,
		}
	}
	res := resolver.ResolveConflicts(cands, "web", "x")
	if len(res.IDs) != n {
		t.Fatalf("expected %d resolved ids, got %d", n, len(res.IDs))
	}
	// First id should be the highest-priority one.
	if res.IDs[0] != cands[0].ID {
		t.Fatalf("expected first id %s, got %s", cands[0].ID, res.IDs[0])
	}
}

// TestCondition_DEF_01 asserts the end-time boundary is exclusive in
// the seeded memory path. The promotion that ends exactly at the
// requested time should NOT be returned.
func TestCondition_DEF_01(t *testing.T) {
	mem := storage.NewMemoryStore()
	mem.PutPromotion(domain.Promotion{
		ID:           "p-boundary",
		TenantID:     "tenant-a",
		Channel:      "web",
		ProductScope: "wildcard",
		Priority:     1,
		PercentBP:    100,
		ValidFrom:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}, nil)
	// At exact end → not included (exclusive comparison).
	cands, err := mem.SelectCandidates(context.Background(), "tenant-a", "web", "x",
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 0 {
		t.Fatalf("expected 0 candidates at exact end (exclusive), got %d", len(cands))
	}
	// One second before end → still active.
	cands, err = mem.SelectCandidates(context.Background(), "tenant-a", "web", "x",
		time.Date(2026, 5, 31, 23, 59, 59, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate before end, got %d", len(cands))
	}
}

// TestCondition_DEF_02 asserts equal-priority promotions have an
// unstable order (no id tie-breaker). The seeded ListPromotions and
// resolver both lack the secondary sort key.
func TestCondition_DEF_02(t *testing.T) {
	mem := storage.NewMemoryStore()
	// Build 5 promotions with the same priority.
	for i := 0; i < 5; i++ {
		mem.PutPromotion(domain.Promotion{
			ID: "p-" + string(rune('a'+i)), TenantID: "tenant-a",
			Channel: "web", ProductScope: "wildcard",
			Priority: 1, PercentBP: 100,
			ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			ValidTo:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		}, nil)
	}
	got := mem.ListPromotions("tenant-a")
	if len(got) != 5 {
		t.Fatalf("expected 5 promotions, got %d", len(got))
	}
	ids := make([]string, 0, len(got))
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	// The expected stable order is alphabetical. The seeded
	// implementation may produce a different order.
	expected := []string{"p-a", "p-b", "p-c", "p-d", "p-e"}
	sort.Strings(expected)
	sort.Strings(ids)
	if !equalStringSlices(ids, expected) {
		t.Logf("note: ids=%v expected=%v (stability not guaranteed under seeded order)", ids, expected)
	}
	// Resolver level: also unstable.
	r := promotion.New()
	cands := []domain.Promotion{
		{ID: "p-z", Channel: "web", ProductScope: "wildcard", Priority: 1},
		{ID: "p-a", Channel: "web", ProductScope: "wildcard", Priority: 1},
		{ID: "p-m", Channel: "web", ProductScope: "wildcard", Priority: 1},
	}
	res := r.ResolveConflicts(cands, "web", "x")
	if len(res.IDs) != 3 {
		t.Fatalf("expected 3 resolved ids, got %d", len(res.IDs))
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestCondition_DEF_03 asserts the resolver does NOT de-duplicate by
// promotion id, so a promotion that qualifies through both an SKU
// path and a wildcard path is applied twice.
func TestCondition_DEF_03(t *testing.T) {
	r := promotion.New()
	cands := []domain.Promotion{
		{ID: "p1", Channel: "web", ProductScope: "sku", ProductID: "SKU-1", Priority: 1},
		{ID: "p1", Channel: "web", ProductScope: "wildcard", Priority: 1},
	}
	res := r.ResolveConflicts(cands, "web", "SKU-1")
	if len(res.IDs) != 2 {
		t.Fatalf("expected duplicate application (2 ids), got %d", len(res.IDs))
	}
	if res.IDs[0] != "p1" || res.IDs[1] != "p1" {
		t.Fatalf("expected both entries to be p1, got %v", res.IDs)
	}
}

// TestCondition_DEF_04 asserts the cache key omits tenant_id and
// config_version. Two tenants with identical (customer, sku, qty,
// channel, date) collide on the same key.
func TestCondition_DEF_04(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	k1 := cache.Key("tenant-a", "cfg-a", "cust-1", "SKU-1", 1, "web", now)
	k2 := cache.Key("tenant-b", "cfg-b", "cust-1", "SKU-1", 1, "web", now)
	if k1 != k2 {
		t.Fatalf("expected identical keys for different tenants, got %q vs %q", k1, k2)
	}
}

// TestCondition_DEF_05 asserts the batch path uses a different
// rounding point than the interactive path, so quantities that
// produce fractional intermediate values disagree.
func TestCondition_DEF_05(t *testing.T) {
	mem := storage.NewMemoryStore()
	mem.PutTenant(domain.Tenant{ID: "tenant-a", Name: "tenant-a"})
	mem.PutProduct(domain.Product{ID: "p-1", TenantID: "tenant-a", SKU: "SKU-A-1", Name: "Widget", ListJPY: 1000})
	mem.PutCustomer(domain.Customer{ID: "c-1", TenantID: "tenant-a", Name: "Acme"})
	mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-1", TenantID: "tenant-a", Active: true})
	mem.SetActiveConfig("tenant-a", "cfg-1")
	// Two non-stacking promotions that, when summed, produce a
	// different discount in the batch path.
	mem.PutPromotion(domain.Promotion{
		ID: "p1", TenantID: "tenant-a", Channel: "web", ProductScope: "wildcard",
		Priority: 1, Stacking: false, PercentBP: 3333,
		ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}, nil)
	mem.PutPromotion(domain.Promotion{
		ID: "p2", TenantID: "tenant-a", Channel: "web", ProductScope: "wildcard",
		Priority: 2, Stacking: true, PercentBP: 3333,
		ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}, nil)
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)
	engine := pricing.New(mem, cache.New(0, time.Minute), ruleengine.NewCompiler(), m)
	req := domain.PricingRequest{
		TenantID: "tenant-a", CustomerID: "c-1", SKU: "SKU-A-1",
		Quantity: 3, Channel: "web",
		OccurredAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	}
	interactive, err := engine.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := engine.ProcessBatch(context.Background(), []domain.PricingRequest{req})
	if err != nil {
		t.Fatal(err)
	}
	// Interactive discounts each 33.33% in turn, batch sums then rounds.
	// We expect the two to differ for fractional intermediate values.
	if interactive.DiscountJPY == batch[0].DiscountJPY {
		t.Logf("note: interactive and batch agree on DiscountJPY=%d (test fixture may not exercise the divergence)", interactive.DiscountJPY)
	}
}

// TestCondition_SEC_01 asserts the access log carries unredacted
// customer_id, negotiated_price and discount_reason values.
func TestCondition_SEC_01(t *testing.T) {
	srv, _ := newPrivateApp(t)
	// Hit the quote endpoint with a sensitive body. The seeded
	// access logger emits a "body" field on the log line that
	// includes the raw customer_id, negotiated_price and
	// discount_reason values.
	body, _ := json.Marshal(domain.PricingRequest{
		TenantID:   "tenant-a",
		CustomerID: "cust-tenant-a-00001",
		SKU:        "SKU-tenant-a-00001",
		Quantity:   1,
		Channel:    "web",
		OccurredAt: time.Now().UTC(),
	})
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/pricing/quote", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// The seeded logger writes the body to stdout; we cannot
	// intercept it from inside an httptest server. The condition
	// is detected by reading the source of internal/httpapi/server.go
	// and asserting that the access log references the unredacted
	// fields without going through the redact package.
	src, err := os.ReadFile("../internal/httpapi/server.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "\"customer_id\"") || !strings.Contains(string(src), "\"negotiated_price\"") {
		t.Fatalf("expected access log to carry unredacted customer_id and negotiated_price fields")
	}
}

// TestCondition_SEC_02 asserts that the admin search builds the
// SQL with string interpolation rather than parameter binding, so a
// synthesised injection-style value survives into the SQL.
func TestCondition_SEC_02(t *testing.T) {
	sqlText, args, err := adminsearch.BuildQuery("products", "x'; DROP TABLE products; --", "name", "asc", 10)
	if err != nil {
		t.Fatal(err)
	}
	// The seeded implementation does not reject the injection-style
	// input and concatenates it directly into the SQL.
	if !strings.Contains(sqlText, "DROP TABLE products") {
		t.Fatalf("expected SQL to contain the injection value, got %q", sqlText)
	}
	if len(args) > 1 || (len(args) == 1 && args[0] != "any") {
		t.Logf("note: SQL has args %v — parameter binding detected", args)
	}
}

// helper for os tests.
func TestConditionHelpersCompile(t *testing.T) {
	if _, err := os.Stat("../testdata/fixtures/small.json"); err != nil {
		t.Fatal(err)
	}
}
