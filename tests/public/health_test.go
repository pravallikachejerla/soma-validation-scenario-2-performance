// Package public contains the normal-engineering tests that run on
// every build. They are designed to pass on the clean baseline and to
// keep passing on the seeded source for everything that does not
// affect visible behaviour.
package public_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/application"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/httpapi"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// newTestServer wires a fresh in-memory app behind an httptest server.
func newTestServer(t *testing.T) (*httptest.Server, *storage.MemoryStore) {
	t.Helper()
	mem := storage.NewMemoryStore()
	mem.PutTenant(domain.Tenant{ID: "tenant-a", Name: "tenant-a"})
	mem.PutProduct(domain.Product{ID: "p-1", TenantID: "tenant-a", SKU: "SKU-A-1", Name: "Widget", ListJPY: 5000})
	mem.PutCustomer(domain.Customer{ID: "c-1", TenantID: "tenant-a", Name: "Acme"})
	mem.PutConfigVersion(domain.ConfigVersion{ID: "cfg-1", TenantID: "tenant-a", Active: true})
	mem.SetActiveConfig("tenant-a", "cfg-1")
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)
	app := application.New(mem, cache.New(64, time.Minute), m, application.BuildIdentity{Commit: "test", BuiltAt: "now"})
	srv := httptest.NewServer(httpapi.New(app))
	t.Cleanup(srv.Close)
	return srv, mem
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVersionEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/version")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var v map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	if v["commit"] != "test" {
		t.Fatalf("expected commit=test, got %v", v["commit"])
	}
}
