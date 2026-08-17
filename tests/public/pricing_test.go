package public_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// TestQuoteHappyPath exercises a non-overlapping interactive quote
// against a tiny dataset so the seeded conditions that require
// overlapping promotions or repeated expressions cannot mask failures.
func TestQuoteHappyPath(t *testing.T) {
	srv, mem := newTestServer(t)
	// Add a single, non-stacking, non-overlapping promotion.
	mem.PutPromotion(domain.Promotion{
		ID:           "prom-1",
		TenantID:     "tenant-a",
		Name:         "Welcome",
		Channel:      "web",
		ProductScope: "wildcard",
		Priority:     1,
		Stacking:     false,
		Exclusion:    false,
		PercentBP:    500, // 5%
		ValidFrom:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:      time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}, nil)
	body, _ := json.Marshal(domain.PricingRequest{
		TenantID:   "tenant-a",
		CustomerID: "c-1",
		SKU:        "SKU-A-1",
		Quantity:   1,
		Channel:    "web",
		OccurredAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
	})
	resp, err := http.Post(srv.URL+"/api/v1/pricing/quote", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var d domain.PricingDecision
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatal(err)
	}
	// 5% off a 5000 JPY list price = 4750.
	if d.AmountJPY != 4750 {
		t.Fatalf("expected amount 4750, got %d", d.AmountJPY)
	}
	if len(d.AppliedPromotionIDs) != 1 || d.AppliedPromotionIDs[0] != "prom-1" {
		t.Fatalf("expected one applied promotion prom-1, got %v", d.AppliedPromotionIDs)
	}
}

func TestQuoteInvalidChannel(t *testing.T) {
	srv, _ := newTestServer(t)
	body, _ := json.Marshal(domain.PricingRequest{
		TenantID:   "tenant-a",
		CustomerID: "c-1",
		SKU:        "SKU-A-1",
		Quantity:   1,
		Channel:    "rogue",
		OccurredAt: time.Now().UTC(),
	})
	resp, err := http.Post(srv.URL+"/api/v1/pricing/quote", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
