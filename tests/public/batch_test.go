package public_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

func TestBatchEndpoint(t *testing.T) {
	srv, mem := newTestServer(t)
	mem.PutPromotion(domain.Promotion{
		ID:           "prom-b1",
		TenantID:     "tenant-a",
		Name:         "batch-discount",
		Channel:      "web",
		ProductScope: "wildcard",
		Priority:     1,
		Stacking:     false,
		PercentBP:    1000, // 10%
		ValidFrom:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:      time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}, nil)
	body, _ := json.Marshal(map[string]any{
		"tenant_id": "tenant-a",
		"items": []domain.PricingRequest{
			{TenantID: "tenant-a", CustomerID: "c-1", SKU: "SKU-A-1", Quantity: 2, Channel: "web", OccurredAt: time.Now().UTC()},
			{TenantID: "tenant-a", CustomerID: "c-1", SKU: "SKU-A-1", Quantity: 3, Channel: "web", OccurredAt: time.Now().UTC()},
		},
	})
	resp, err := http.Post(srv.URL+"/api/v1/pricing/batch", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out struct {
		Decisions []domain.PricingDecision `json:"decisions"`
		Summary   map[string]any           `json:"summary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(out.Decisions))
	}
}
