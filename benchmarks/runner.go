// Package benchmarks exposes the developer benchmark runner as a
// library so other tools can call it programmatically.
package benchmarks

import (
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/application"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/seeddata"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"

	"github.com/prometheus/client_golang/prometheus"
)

// Result is the per-run summary.
type Result struct {
	Profile        string        `json:"profile"`
	Rounds         int           `json:"rounds"`
	TotalDuration  time.Duration `json:"total_duration"`
	AvgDuration    time.Duration `json:"avg_duration"`
	Queries        int64         `json:"queries"`
	Compiles       int64         `json:"compiles"`
	CacheHits      int64         `json:"cache_hits"`
	CandidateCount int64         `json:"candidate_count"`
}

// RunProfile exercises the in-memory pricing engine for a given
// profile and number of rounds.
func RunProfile(profile string, rounds int) (Result, error) {
	profiles := map[string]seeddata.Profile{
		"small":  seeddata.Small,
		"medium": seeddata.Medium,
		"large":  seeddata.Large,
	}
	p, ok := profiles[profile]
	if !ok {
		return Result{}, domain.ErrInvalidInput
	}
	ds := seeddata.Build(int64(seeddata.DefaultSeed), p)
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
	app := application.New(mem, cache.New(0, time.Minute), m, application.BuildIdentity{Commit: "bench"})
	tenant := ds.TenantIDs[0]
	sku := ds.Products[0].SKU
	cust := ds.Customers[0].ID
	started := time.Now()
	for i := 0; i < rounds; i++ {
		_, err := app.Engine.Evaluate(nil, domain.PricingRequest{
			TenantID:   tenant,
			CustomerID: cust,
			SKU:        sku,
			Quantity:   1,
			Channel:    "web",
			OccurredAt: time.Now().UTC(),
		})
		if err != nil {
			return Result{}, err
		}
	}
	dur := time.Since(started)
	return Result{
		Profile:        profile,
		Rounds:         rounds,
		TotalDuration:  dur,
		AvgDuration:    dur / time.Duration(rounds),
		Queries:        m.QueryCountValue(),
		Compiles:       m.CompileCountValue(),
		CacheHits:      m.CacheHitsValue(),
		CandidateCount: m.CandidateCountValue(),
	}, nil
}
