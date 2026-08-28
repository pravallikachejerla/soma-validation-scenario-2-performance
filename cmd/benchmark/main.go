package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/application"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/seeddata"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"
)

func main() {
	profile := flag.String("profile", "medium", "small|medium|large")
	rounds := flag.Int("rounds", 100, "number of pricing rounds per worker")
	flag.Parse()

	profiles := map[string]seeddata.Profile{
		"small": seeddata.Small, "medium": seeddata.Medium, "large": seeddata.Large,
	}
	p := profiles[*profile]
	ds := seeddata.Build(int64(seeddata.DefaultSeed), p)

	mem := storage.NewMemoryStore()
	for _, t := range ds.TenantIDs {
		mem.PutTenant(domain.Tenant{ID: t, Name: t})
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
		mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-" + pr.TenantID, TenantID: pr.TenantID, Label: "default", Active: true})
		mem.SetActiveConfig(pr.TenantID, "cfg-"+pr.TenantID)
	}

	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)
	app := application.New(mem, cache.New(0, time.Minute), m, application.BuildIdentity{Commit: "dev"})
	tenant := ds.TenantIDs[0]
	sku := ds.Products[0].SKU
	cust := ds.Customers[0].ID
	started := time.Now()
	for i := 0; i < *rounds; i++ {
		_, err := app.Engine.Evaluate(nil, domain.PricingRequest{
			TenantID:   tenant,
			CustomerID: cust,
			SKU:        sku,
			Quantity:   1,
			Channel:    "web",
			OccurredAt: time.Now().UTC(),
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	dur := time.Since(started)
	fmt.Printf("profile=%s rounds=%d total=%s avg=%s queries=%d compiles=%d cache_hits=%d candidate_count=%d\n",
		*profile, *rounds, dur, dur/time.Duration(*rounds),
		m.QueryCountValue(), m.CompileCountValue(), m.CacheHitsValue(), m.CandidateCountValue())
}
