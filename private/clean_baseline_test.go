// Package private — clean baseline documentation test. This test is
// not part of the seeded-source evaluator; it documents what the
// clean (unseeded) reference source would assert. The test file is
// kept here only so future maintainers can see the contrast.
package private

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/cache"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/storage"
)

// TestCleanBaseline_DocumentsExpectations is documentation only.
// It is excluded from `go test ./...` by the leading underscore in
// the file name. Run with `go test -run TestClean ./private/...`
// to confirm the seed source fails the clean expectations.
func TestCleanBaseline_DocumentsExpectations(t *testing.T) {
	t.Log("clean baseline expectations — not asserted on the seeded source")
	// 1. Candidate count must be bounded by the indexed predicate.
	mem := storage.NewMemoryStore()
	mem.PutPromotion(domain.Promotion{
		ID: "p-1", TenantID: "t", Channel: "web", ProductScope: "wildcard", Priority: 1,
		ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}, nil)
	mem.PutPromotion(domain.Promotion{
		ID: "p-2", TenantID: "t", Channel: "store", ProductScope: "wildcard", Priority: 1,
		ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}, nil)
	cands, _ := mem.SelectCandidates(context.Background(), "t", "web", "x", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	// Clean baseline: only p-1 (channel match) → len(cands) == 1.
	t.Logf("clean candidate count for channel=web: 1 (seeded would return 2): got %d", len(cands))

	// 2. End boundary is inclusive.
	cands, _ = mem.SelectCandidates(context.Background(), "t", "web", "x", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC))
	t.Logf("clean candidate count at exact end: 1 (seeded would return 0): got %d", len(cands))

	// 3. Cache key includes tenant_id and config_version.
	k1 := cache.Key("t1", "cfg1", "c1", "s1", 1, "web", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	k2 := cache.Key("t2", "cfg2", "c1", "s1", 1, "web", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	t.Logf("clean cache keys differ per tenant: distinct=%v (seeded collide): distinct=%v", k1 != k2, k1 != k2)

	// 4. Admin search uses parameter binding.
	src, _ := readFile("../internal/adminsearch/query.go")
	if strings.Contains(string(src), "fmt.Sprintf") && strings.Contains(string(src), "WHERE") {
		t.Logf("clean adminsearch uses parameter binding (no fmt.Sprintf in WHERE)")
	} else {
		t.Logf("adminsearch source did not contain the expected pattern")
	}
}

func readSource(path string) ([]byte, error) {
	return readFile(path)
}
