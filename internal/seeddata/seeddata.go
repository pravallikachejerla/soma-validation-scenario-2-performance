// Package seeddata builds deterministic synthetic data fixtures used by
// the seeder and the public tests.
package seeddata

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// Profile controls the volume of generated data.
type Profile struct {
	Name             string
	Tenants          int
	ProductsPerTenant int
	CustomersPerTenant int
	PromotionsPerTenant int
	OverlapPct       int // for the large profile
}

// DefaultSeed is the deterministic PRNG seed.
const DefaultSeed = 42

// Small is used by the public test suite and the smoke script.
var Small = Profile{
	Name:               "small",
	Tenants:            2,
	ProductsPerTenant:  20,
	CustomersPerTenant: 10,
	PromotionsPerTenant: 10,
	OverlapPct:         0,
}

// Medium is used by the seeder and the developer demo.
var Medium = Profile{
	Name:               "medium",
	Tenants:            3,
	ProductsPerTenant:  200,
	CustomersPerTenant: 50,
	PromotionsPerTenant: 100,
	OverlapPct:         0,
}

// Large is used by the private benchmarks.
var Large = Profile{
	Name:               "large",
	Tenants:            10,
	ProductsPerTenant:  1000,
	CustomersPerTenant: 200,
	PromotionsPerTenant: 500,
	OverlapPct:         70,
}

// Dataset is the serialised shape written to disk.
type Dataset struct {
	Profile     Profile            `json:"profile"`
	TenantIDs   []string           `json:"tenant_ids"`
	Products    []domain.Product   `json:"products"`
	Customers   []domain.Customer  `json:"customers"`
	Promotions  []domain.Promotion `json:"promotions"`
	Conditions  []domain.PromotionCondition `json:"conditions"`
}

// Build generates a deterministic dataset.
func Build(seed int64, p Profile) *Dataset {
	r := rand.New(rand.NewSource(seed))
	ds := &Dataset{Profile: p}
	for i := 0; i < p.Tenants; i++ {
		ds.TenantIDs = append(ds.TenantIDs, fmt.Sprintf("tenant-%c", 'a'+i))
	}
	// Products
	pid := 0
	for _, t := range ds.TenantIDs {
		for i := 0; i < p.ProductsPerTenant; i++ {
			pid++
			ds.Products = append(ds.Products, domain.Product{
				ID:       fmt.Sprintf("prod-%s-%05d", t, pid),
				TenantID: t,
				SKU:      fmt.Sprintf("SKU-%s-%05d", t, pid),
				Name:     fmt.Sprintf("Product %s %d", t, pid),
				ListJPY:  int64(1000 + r.Intn(9000)),
				Category: pickCategory(r),
			})
		}
	}
	// Customers
	cid := 0
	for _, t := range ds.TenantIDs {
		for i := 0; i < p.CustomersPerTenant; i++ {
			cid++
			ds.Customers = append(ds.Customers, domain.Customer{
				ID:       fmt.Sprintf("cust-%s-%05d", t, cid),
				TenantID: t,
				Name:     fmt.Sprintf("Customer %s %d", t, cid),
				Segment:  pickSegment(r),
			})
		}
	}
	// Promotions
	prid := 0
	condid := 0
	channels := []string{"web", "store", "partner", "api", "wholesale"}
	for _, t := range ds.TenantIDs {
		for i := 0; i < p.PromotionsPerTenant; i++ {
			prid++
			scope := "wildcard"
			productID := ""
			if r.Intn(100) < 50 {
				scope = "sku"
				idx := r.Intn(p.ProductsPerTenant)
				productID = fmt.Sprintf("prod-%s-%05d", t, idx+1+offsetForTenant(ds.TenantIDs, t))
			}
			prom := domain.Promotion{
				ID:           fmt.Sprintf("prom-%s-%05d", t, prid),
				TenantID:     t,
				Name:         fmt.Sprintf("Promotion %s %d", t, prid),
				Channel:      channels[r.Intn(len(channels))],
				ProductID:    productID,
				ProductScope: scope,
				Priority:     1 + r.Intn(10),
				Stacking:     r.Intn(2) == 0,
				Exclusion:    false,
				PercentBP:    100 + r.Intn(1500),
				ValidFrom:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				ValidTo:      time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
			}
			ds.Promotions = append(ds.Promotions, prom)
			if r.Intn(100) < 50 {
				condid++
				ds.Conditions = append(ds.Conditions, domain.PromotionCondition{
					ID:          fmt.Sprintf("cond-%05d", condid),
					PromotionID: prom.ID,
					Kind:        "quantity",
					Expr:        "quantity >= 1",
				})
			}
		}
	}
	return ds
}

func pickCategory(r *rand.Rand) string {
	cats := []string{"hardware", "consumable", "service", "subscription"}
	return cats[r.Intn(len(cats))]
}

func pickSegment(r *rand.Rand) string {
	segs := []string{"standard", "premium", "enterprise", "internal"}
	return segs[r.Intn(len(segs))]
}

func offsetForTenant(tenants []string, t string) int {
	for i, n := range tenants {
		if n == t {
			return i * 1_000_000
		}
	}
	return 0
}

// Save writes the dataset as JSON to path.
func (d *Dataset) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
