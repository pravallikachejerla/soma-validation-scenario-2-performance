package public_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

func TestListPromotionsByTenant(t *testing.T) {
	srv, mem := newTestServer(t)
	for i := 0; i < 3; i++ {
		mem.PutPromotion(domain.Promotion{
			ID:           "p-" + string(rune('a'+i)),
			TenantID:     "tenant-a",
			Name:         "promo",
			Channel:      "web",
			ProductScope: "wildcard",
			Priority:     1,
			PercentBP:    200,
			ValidFrom:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			ValidTo:      time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
		}, nil)
	}
	resp, err := http.Get(srv.URL + "/api/v1/promotions?tenant_id=tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	arr, _ := out["promotions"].([]any)
	if len(arr) != 3 {
		t.Fatalf("expected 3 promotions, got %d", len(arr))
	}
}

func TestCreatePromotion(t *testing.T) {
	srv, _ := newTestServer(t)
	body, _ := json.Marshal(domain.Promotion{
		TenantID:     "tenant-a",
		Name:         "fresh",
		Channel:      "web",
		ProductScope: "wildcard",
		Priority:     2,
		PercentBP:    300,
		ValidFrom:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:      time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	})
	resp, err := http.Post(srv.URL+"/api/v1/promotions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAdminSearch(t *testing.T) {
	srv, mem := newTestServer(t)
	mem.PutProduct(domain.Product{ID: "p-2", TenantID: "tenant-a", SKU: "SKU-A-2", Name: "Gizmo", ListJPY: 3000})
	q := url.Values{"entity": {"products"}, "q": {"Giz"}, "sort": {"name"}, "order": {"asc"}}
	resp, err := http.Get(srv.URL + "/api/v1/admin/search?" + q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
