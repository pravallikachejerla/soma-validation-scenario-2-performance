package pricing_test

// Differential (golden-master) equivalence test.
//
// The golden file internal/pricing/testdata/equivalence_golden.json is
// produced ONCE from the frozen baseline commit (70b7e68) with
//
//	SOMA_UPDATE_GOLDEN=1 go test ./internal/pricing -run TestEquivalenceUpdateGolden
//
// run inside the pristine baseline worktree. Every candidate worktree then
// replays the identical request set and must reproduce the golden values with
// 0 differences: AmountJPY, DiscountJPY, SubtotalJPY, ordered
// AppliedPromotionIDs and the error class.
//
// Interactive (Evaluate) and batch (ProcessBatch) are recorded and compared
// separately, each against its own baseline, so the known F7 rounding
// divergence between the two paths is preserved rather than "fixed".
//
// Exact equality is only asserted on tie-free data (unique priority per
// tenant): the baseline orders equal-priority promotions by map iteration, so
// tied data has no single correct answer. Tied data (the unmodified seeded
// priorities) is therefore compared on AmountJPY only, and only for requests
// whose amount was identical across many baseline passes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/pricing"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/ruleengine"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/seeddata"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"
)

const (
	baseCommit = "70b7e68"
	goldenPath = "testdata/equivalence_golden.json"
	// tiePasses is the number of baseline passes used to decide whether a
	// tied request has a stable amount. 2^-(passes-1) is the chance that a
	// genuinely random two-way tie looks stable by luck.
	tiePasses = 15
)

var (
	allChannels = []string{"web", "store", "partner", "api", "wholesale"}
	evalAt      = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
)

// rec is the compared slice of a decision (or the error class).
type rec struct {
	Amount   int64  `json:"a"`
	Discount int64  `json:"d"`
	Subtotal int64  `json:"s"`
	IDs      string `json:"p"`
	Err      string `json:"e"`
}

type goldenFile struct {
	BaseCommit string           `json:"base_commit"`
	Exact      map[string]rec   `json:"exact"`
	TieAmounts map[string]int64 `json:"tie_amounts"`
}

type profileCase struct {
	name    string
	profile seeddata.Profile
	skus    int
}

// Small: 3 tenant/sku pairs, medium and large: 2. Quantities 1..50 and every
// channel are crossed with each pair.
var profileCases = []profileCase{
	{"small", seeddata.Small, 3},
	{"medium", seeddata.Medium, 2},
	{"large", seeddata.Large, 2},
}

type keyedReq struct {
	key string
	req domain.PricingRequest
}

func updating() bool { return os.Getenv("SOMA_UPDATE_GOLDEN") == "1" }

// newEngine builds an engine over an in-memory store seeded from ds. When
// tieFree is set every tenant's promotions get unique priorities 1..n.
func newEngine(ds *seeddata.Dataset, tieFree bool, pc *cache.PricingCache) *pricing.Engine {
	mem := storage.NewMemoryStore()
	for _, t := range ds.TenantIDs {
		mem.PutTenant(domain.Tenant{ID: t, Name: t})
		mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-" + t, TenantID: t, Label: "default", Active: true})
		mem.SetActiveConfig(t, "cfg-"+t)
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
	prio := map[string]int{}
	for _, p := range ds.Promotions {
		if tieFree {
			prio[p.TenantID]++
			p.Priority = prio[p.TenantID]
		}
		mem.PutPromotion(p, conds[p.ID])
	}
	m := observability.NewMetrics(prometheus.NewRegistry())
	return pricing.New(mem, pc, ruleengine.NewCompiler(), m)
}

// requestsFor lists the deterministic request set for a dataset.
func requestsFor(ds *seeddata.Dataset, skus int, withErrors bool) []keyedReq {
	var out []keyedReq
	for k := 0; k < skus; k++ {
		tn := ds.TenantIDs[k%len(ds.TenantIDs)]
		nth := k / len(ds.TenantIDs)
		var sku, cust string
		seen := 0
		for _, p := range ds.Products {
			if p.TenantID == tn {
				if seen == nth {
					sku = p.SKU
					break
				}
				seen++
			}
		}
		for _, c := range ds.Customers {
			if c.TenantID == tn {
				cust = c.ID
				break
			}
		}
		for qty := 1; qty <= 50; qty++ {
			for _, ch := range allChannels {
				key := fmt.Sprintf("%s|%s|q%d|%s", tn, sku, qty, ch)
				out = append(out, keyedReq{key, domain.PricingRequest{
					ID: key, TenantID: tn, CustomerID: cust, SKU: sku,
					Quantity: qty, Channel: ch, OccurredAt: evalAt,
				}})
			}
		}
		if withErrors && k == 0 {
			base := domain.PricingRequest{ID: "err", TenantID: tn, CustomerID: cust, SKU: sku, Quantity: 1, Channel: "web", OccurredAt: evalAt}
			mut := func(name string, f func(r *domain.PricingRequest)) {
				r := base
				f(&r)
				out = append(out, keyedReq{"err:" + name, r})
			}
			mut("unknown_sku", func(r *domain.PricingRequest) { r.SKU = "NO-SUCH-SKU" })
			mut("unknown_customer", func(r *domain.PricingRequest) { r.CustomerID = "nobody" })
			mut("unknown_tenant", func(r *domain.PricingRequest) { r.TenantID = "tenant-zzz" })
			mut("qty_zero", func(r *domain.PricingRequest) { r.Quantity = 0 })
			mut("qty_negative", func(r *domain.PricingRequest) { r.Quantity = -1 })
			mut("empty_channel", func(r *domain.PricingRequest) { r.Channel = "" })
		}
	}
	return out
}

func errClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, domain.ErrNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrInvalidInput):
		return "invalid_input"
	case errors.Is(err, domain.ErrNoConfigVersion):
		return "no_config_version"
	case errors.Is(err, domain.ErrCrossTenant):
		return "cross_tenant"
	case errors.Is(err, domain.ErrConflict):
		return "conflict"
	case errors.Is(err, domain.ErrUnauthorized):
		return "unauthorized"
	}
	return "other"
}

func toRec(d domain.PricingDecision, err error) rec {
	if err != nil {
		return rec{Err: errClass(err)}
	}
	return rec{
		Amount: d.AmountJPY, Discount: d.DiscountJPY, Subtotal: d.SubtotalJPY,
		IDs: strings.Join(d.AppliedPromotionIDs, ","),
	}
}

func runInteractive(e *pricing.Engine, r domain.PricingRequest) rec {
	d, err := e.Evaluate(context.Background(), r)
	return toRec(d, err)
}

func runBatch(e *pricing.Engine, r domain.PricingRequest) rec {
	ds, err := e.ProcessBatch(context.Background(), []domain.PricingRequest{r})
	if err != nil || len(ds) != 1 {
		if err == nil {
			err = errors.New("batch returned no decision")
		}
		return rec{Err: errClass(err)}
	}
	return toRec(ds[0], nil)
}

// collect replays the whole request set on the engine code under test.
func collect(passes int) goldenFile {
	g := goldenFile{BaseCommit: baseCommit, Exact: map[string]rec{}, TieAmounts: map[string]int64{}}
	for _, pc := range profileCases {
		ds := seeddata.Build(int64(seeddata.DefaultSeed), pc.profile)
		e := newEngine(ds, true, nil)
		for _, kr := range requestsFor(ds, pc.skus, true) {
			g.Exact["interactive|"+pc.name+"|"+kr.key] = runInteractive(e, kr.req)
			g.Exact["batch|"+pc.name+"|"+kr.key] = runBatch(e, kr.req)
		}
	}

	// Tied data: the unmodified seeded priorities on the medium profile.
	ds := seeddata.Build(int64(seeddata.DefaultSeed), seeddata.Medium)
	e := newEngine(ds, false, nil)
	reqs := requestsFor(ds, 1, false)
	stable := map[string]bool{}
	for _, kr := range reqs {
		a := runInteractive(e, kr.req).Amount
		g.TieAmounts["medium|"+kr.key] = a
		stable["medium|"+kr.key] = true
	}
	for p := 1; p < passes; p++ {
		for _, kr := range reqs {
			k := "medium|" + kr.key
			if runInteractive(e, kr.req).Amount != g.TieAmounts[k] {
				stable[k] = false
			}
		}
	}
	for k, ok := range stable {
		if !ok {
			delete(g.TieAmounts, k)
		}
	}
	return g
}

var (
	collectOnce sync.Once
	collected   goldenFile
)

func replay() goldenFile {
	collectOnce.Do(func() { collected = collect(1) })
	return collected
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	b, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file missing (%v); generate it on the baseline worktree with SOMA_UPDATE_GOLDEN=1 go test ./internal/pricing -run TestEquivalenceUpdateGolden", err)
	}
	var g goldenFile
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("golden file unreadable: %v", err)
	}
	if g.BaseCommit != baseCommit {
		t.Fatalf("golden was frozen from %q, expected %q", g.BaseCommit, baseCommit)
	}
	return g
}

// TestEquivalenceUpdateGolden freezes the baseline behaviour. It is a no-op
// unless SOMA_UPDATE_GOLDEN=1 and must only be run on the pristine baseline.
func TestEquivalenceUpdateGolden(t *testing.T) {
	if !updating() {
		t.Skip("set SOMA_UPDATE_GOLDEN=1 on the baseline worktree to regenerate")
	}
	g := collect(tiePasses)
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goldenPath, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("froze %d exact records and %d stable tie amounts", len(g.Exact), len(g.TieAmounts))
}

func compareExact(t *testing.T, prefix string) {
	t.Helper()
	if updating() {
		t.Skip("golden update run")
	}
	want, got := loadGolden(t), replay()
	n, bad := 0, 0
	for k, w := range want.Exact {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		n++
		if g, ok := got.Exact[k]; !ok || g != w {
			bad++
			if bad <= 5 {
				t.Errorf("%s: want %+v got %+v", k, w, g)
			}
		}
	}
	if n == 0 {
		t.Fatalf("golden holds no %q records", prefix)
	}
	if bad > 0 {
		t.Fatalf("%d of %d %s decisions differ from baseline", bad, n, prefix)
	}
	t.Logf("%d %s decisions identical to baseline (0 differences)", n, prefix)
}

func TestEquivalenceInteractive(t *testing.T) { compareExact(t, "interactive|") }

func TestEquivalenceBatch(t *testing.T) { compareExact(t, "batch|") }

// TestEquivalenceTiedAmounts compares amount only, for tied data.
func TestEquivalenceTiedAmounts(t *testing.T) {
	if updating() {
		t.Skip("golden update run")
	}
	want, got := loadGolden(t), replay()
	if len(want.TieAmounts) == 0 {
		t.Fatal("golden holds no stable tie amounts")
	}
	bad := 0
	for k, w := range want.TieAmounts {
		if g := got.TieAmounts[k]; g != w {
			bad++
			if bad <= 5 {
				t.Errorf("%s: want amount %d got %d", k, w, g)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d of %d tied amounts differ from baseline", bad, len(want.TieAmounts))
	}
}

// TestConcurrentEvaluateMatchesGolden drives one engine (with a small,
// evicting cache) from several goroutines. It is the evaluator that gives
// `go test -race` something to find once the global evaluation mutex is
// removed, and it still demands the baseline's exact decisions.
func TestConcurrentEvaluateMatchesGolden(t *testing.T) {
	if updating() {
		t.Skip("golden update run")
	}
	want := loadGolden(t)
	ds := seeddata.Build(int64(seeddata.DefaultSeed), seeddata.Small)
	e := newEngine(ds, true, cache.New(64, time.Minute))
	reqs := requestsFor(ds, 3, false)

	var wg sync.WaitGroup
	var mu sync.Mutex
	bad := 0
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range reqs {
				kr := reqs[(i+w*37)%len(reqs)]
				got := runInteractive(e, kr.req)
				if got != want.Exact["interactive|small|"+kr.key] {
					mu.Lock()
					bad++
					mu.Unlock()
				}
			}
		}(w)
	}
	wg.Wait()
	if bad > 0 {
		t.Fatalf("%d concurrent decisions differ from baseline", bad)
	}
}
