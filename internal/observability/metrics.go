package observability

import (
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

// Metrics is the in-process counter/timer set used by the pricing engine.
// Counters are exposed to Prometheus via the registry.
type Metrics struct {
	QueryCount    prometheus.Counter
	CompileCount  prometheus.Counter
	EvaluateTimer prometheus.Histogram
	CacheHits     prometheus.Counter
	CandidateCount prometheus.Histogram

	queryCount    atomic.Int64
	compileCount  atomic.Int64
	cacheHits     atomic.Int64
	candidateCount atomic.Int64
	mu            sync.Mutex
}

// NewMetrics registers metrics in reg.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		QueryCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pricing_query_count",
			Help: "Total database queries during pricing requests.",
		}),
		CompileCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pricing_compile_count",
			Help: "Total rule expression compilations.",
		}),
		EvaluateTimer: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "pricing_evaluate_duration_seconds",
			Help:    "Duration of pricing evaluation.",
			Buckets: prometheus.DefBuckets,
		}),
		CacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pricing_cache_hits_total",
			Help: "Total pricing cache hits.",
		}),
		CandidateCount: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "pricing_candidate_count",
			Help:    "Promotion candidates examined per request.",
			Buckets: []float64{0, 1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000},
		}),
	}
	if reg != nil {
		reg.MustRegister(m.QueryCount, m.CompileCount, m.EvaluateTimer, m.CacheHits, m.CandidateCount)
	}
	return m
}

// IncQuery increments the in-process and Prometheus query counters.
func (m *Metrics) IncQuery() {
	if m == nil {
		return
	}
	m.queryCount.Add(1)
	m.QueryCount.Inc()
}

// IncCompile increments the in-process and Prometheus compile counters.
func (m *Metrics) IncCompile() {
	if m == nil {
		return
	}
	m.compileCount.Add(1)
	m.CompileCount.Inc()
}

// IncCacheHit increments the in-process and Prometheus cache-hit counters.
func (m *Metrics) IncCacheHit() {
	if m == nil {
		return
	}
	m.cacheHits.Add(1)
	m.CacheHits.Inc()
}

// ObserveCandidate records the candidate count for this request.
func (m *Metrics) ObserveCandidate(n int64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.candidateCount.Store(n)
	m.mu.Unlock()
	m.CandidateCount.Observe(float64(n))
}

// QueryCount returns the in-process query counter for tests.
func (m *Metrics) QueryCountValue() int64 { return m.queryCount.Load() }

// CompileCountValue returns the in-process compile counter.
func (m *Metrics) CompileCountValue() int64 { return m.compileCount.Load() }

// CacheHitsValue returns the in-process cache-hit counter.
func (m *Metrics) CacheHitsValue() int64 { return m.cacheHits.Load() }

// CandidateCountValue returns the last observed candidate count.
func (m *Metrics) CandidateCountValue() int64 { return m.candidateCount.Load() }

// Handler returns an http.Handler that exposes /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}
