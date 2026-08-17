// Package application wires the storage, pricing, and observability
// components into a single App value that the HTTP layer can use.
package application

import (
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/pricing"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/ruleengine"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"
)

// App is the top-level wiring object.
type App struct {
	Engine   *pricing.Engine
	Compiler *ruleengine.Compiler
	Cache    *cache.PricingCache
	Metrics  *observability.Metrics
	MemStore *storage.MemoryStore
	PGStore  *storage.PGStore
	Mode     string // "memory" or "postgres"
	Version  BuildIdentity
}

// BuildIdentity is the immutable build metadata exposed at /version.
type BuildIdentity struct {
	Commit    string `json:"commit"`
	BuiltAt   string `json:"built_at"`
	DatasetID string `json:"dataset_id"`
}

// New returns a fresh App that uses the in-memory store.
func New(mem *storage.MemoryStore, cache *cache.PricingCache, m *observability.Metrics, v BuildIdentity) *App {
	comp := ruleengine.NewCompiler()
	engine := pricing.New(mem, cache, comp, m)
	return &App{
		Engine:   engine,
		Compiler: comp,
		Cache:    cache,
		Metrics:  m,
		MemStore: mem,
		Mode:     "memory",
		Version:  v,
	}
}

// NewPG returns a fresh App that uses the postgres store.
func NewPG(pg *storage.PGStore, cache *cache.PricingCache, m *observability.Metrics, v BuildIdentity) *App {
	comp := ruleengine.NewCompiler()
	engine := pricing.New(pg, cache, comp, m)
	return &App{
		Engine:   engine,
		Compiler: comp,
		Cache:    cache,
		Metrics:  m,
		PGStore:  pg,
		Mode:     "postgres",
		Version:  v,
	}
}
